// Package siq reads SIQ archives without extracting untrusted archive paths.
package siq

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/beevik/etree"
)

const LatestVersion = "5"
const Namespace = "https://github.com/VladimirKhil/SI/blob/master/assets/siq_5.xsd"

type Limits struct {
	Files         int   `yaml:"files"`
	ExpandedBytes int64 `yaml:"expanded_bytes"`
	FileBytes     int64 `yaml:"file_bytes"`
	XMLBytes      int64 `yaml:"xml_bytes"`
}

func DefaultLimits() Limits { return Limits{10000, 4 << 30, 1 << 30, 16 << 20} }

type Media struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Size       int64  `json:"size"`
	Extension  string `json:"extension"`
	References int    `json:"references"`
}

type Analysis struct {
	Name           string   `json:"name"`
	Version        string   `json:"version"`
	Size           int64    `json:"size"`
	ExpandedSize   int64    `json:"expanded_size"`
	Rounds         int      `json:"rounds"`
	Themes         int      `json:"themes"`
	Questions      int      `json:"questions"`
	Media          []Media  `json:"media"`
	Warnings       []string `json:"warnings"`
	OutputVersions []string `json:"output_versions"`
	UnusedFiles    int      `json:"unused_files"`
	UnusedSize     int64    `json:"unused_size"`
}

type reference struct {
	element *etree.Element
	attr    string
	prefix  string
	kind    string
	name    string
}
type Package struct {
	Archive     *zip.ReadCloser
	XML         *etree.Document
	Info        Analysis
	files       map[string]*zip.File
	refs        map[string][]reference
	limits      Limits
	preparedFor string
	usage       *usagePlan
}

func Open(filename string, limits Limits) (_ *Package, err error) {
	z, err := zip.OpenReader(filename)
	if err != nil {
		return nil, fmt.Errorf("open SIQ archive: %w", err)
	}
	p := &Package{Archive: z, files: map[string]*zip.File{}, refs: map[string][]reference{}, limits: limits}
	defer func() {
		if err != nil {
			z.Close()
		}
	}()
	if len(z.File) > limits.Files {
		return nil, fmt.Errorf("archive contains too many files")
	}
	for _, f := range z.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if !safeName(f.Name) || f.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("unsafe archive path: %q", f.Name)
		}
		if _, ok := p.files[f.Name]; ok {
			return nil, fmt.Errorf("duplicate archive entry: %q", f.Name)
		}
		if f.UncompressedSize64 > uint64(limits.FileBytes) {
			return nil, fmt.Errorf("archive entry too large: %q", f.Name)
		}
		p.Info.ExpandedSize += int64(f.UncompressedSize64)
		if p.Info.ExpandedSize > limits.ExpandedBytes {
			return nil, fmt.Errorf("expanded archive is too large")
		}
		p.files[f.Name] = f
	}
	content, err := p.read("content.xml", limits.XMLBytes)
	if err != nil {
		return nil, err
	}
	p.XML, err = parseXML(content)
	if err != nil {
		return nil, err
	}
	root := p.XML.Root()
	if root == nil || root.Tag != "package" {
		return nil, fmt.Errorf("content.xml has no package root")
	}
	v := root.SelectAttrValue("version", "")
	n, err := strconv.ParseFloat(v, 64)
	if err != nil || (n != 3 && n != 3.1 && n != 4 && n != 5) {
		return nil, fmt.Errorf("unsupported package version %q; supported: 3, 3.1, 4, 5", v)
	}
	p.Info.Version = strconv.FormatFloat(n, 'f', -1, 64)
	p.Info.Name = root.SelectAttrValue("name", "")
	p.Info.OutputVersions = []string{LatestVersion, "source"}
	p.Info.Media = []Media{}
	p.Info.Warnings = []string{}
	st, err := os.Stat(filename)
	if err != nil {
		return nil, err
	}
	p.Info.Size = st.Size()
	walk(root, func(e *etree.Element) {
		switch e.Tag {
		case "round":
			p.Info.Rounds++
		case "theme":
			p.Info.Themes++
		case "question":
			p.Info.Questions++
		}
	})
	p.collectReferences()
	plan, err := p.planUsage()
	if err != nil {
		return nil, err
	}
	p.updateUsage(plan)
	return p, nil
}

func (p *Package) updateUsage(plan *usagePlan) {
	p.Info.UnusedFiles = plan.unusedCount
	p.Info.UnusedSize = plan.unusedSize
	p.Info.Media = []Media{}
	for _, f := range p.Archive.File {
		kind := mediaKind(f.Name)
		if !plan.used[f.Name] || kind == "" {
			continue
		}
		count := len(p.refs[f.Name])
		for _, links := range plan.links {
			for _, link := range links {
				if link.target == f.Name {
					count++
				}
			}
		}
		p.Info.Media = append(p.Info.Media, Media{f.Name, kind, int64(f.UncompressedSize64), strings.ToLower(path.Ext(f.Name)), count})
	}
}

func (p *Package) Close() error { return p.Archive.Close() }
func safeName(name string) bool {
	return name != "" && !strings.ContainsAny(name, "\\:\x00") && !strings.HasPrefix(name, "/") && path.Clean(name) == name && name != ".." && !strings.HasPrefix(name, "../")
}
func parseXML(b []byte) (*etree.Document, error) {
	if strings.Contains(strings.ToUpper(string(b)), "<!DOCTYPE") || strings.Contains(strings.ToUpper(string(b)), "<!ENTITY") {
		return nil, fmt.Errorf("DTD and entities are not supported")
	}
	// Bound tree depth before recursive traversal or etree allocation.
	decoder := xml.NewDecoder(bytes.NewReader(b))
	depth, count := 0, 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("invalid XML: %w", err)
		}
		switch token.(type) {
		case xml.StartElement:
			depth++
			count++
			if depth > 128 || count > 500000 {
				return nil, fmt.Errorf("XML complexity limit exceeded")
			}
		case xml.EndElement:
			depth--
		}
	}
	d := etree.NewDocument()
	if err := d.ReadFromBytes(b); err != nil {
		return nil, fmt.Errorf("invalid XML: %w", err)
	}
	return d, nil
}
func (p *Package) read(name string, max int64) ([]byte, error) {
	f, ok := p.files[name]
	if !ok {
		return nil, fmt.Errorf("missing archive entry %s", name)
	}
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	b, err := io.ReadAll(io.LimitReader(r, max+1))
	if int64(len(b)) > max {
		return nil, fmt.Errorf("%s exceeds size limit", name)
	}
	return b, err
}
func walk(e *etree.Element, f func(*etree.Element)) {
	f(e)
	for _, c := range e.ChildElements() {
		walk(c, f)
	}
}
func collection(kind string) string {
	switch kind {
	case "image":
		return "Images"
	case "audio", "voice":
		return "Audio"
	case "video":
		return "Video"
	case "html":
		return "Html"
	}
	return ""
}
func (p *Package) resolve(kind, name string) string {
	dir := collection(kind)
	if dir == "" {
		return ""
	}
	// Prefer the exact ZIP entry, then decode only once. A literal percent is legal.
	for _, candidate := range []string{name, strings.TrimSpace(name)} {
		if _, ok := p.files[dir+"/"+candidate]; ok {
			return dir + "/" + candidate
		}
		if decoded, err := url.PathUnescape(candidate); err == nil {
			if _, ok := p.files[dir+"/"+decoded]; ok {
				return dir + "/" + decoded
			}
		}
		for entry := range p.files {
			if decoded, err := url.PathUnescape(entry); err == nil && decoded == dir+"/"+candidate {
				return entry
			}
		}
	}
	return ""
}
func (p *Package) collectReferences() {
	p.refs = map[string][]reference{}
	add := func(r reference) {
		if name := p.resolve(r.kind, r.name); name != "" {
			p.refs[name] = append(p.refs[name], r)
		} else {
			p.Info.Warnings = append(p.Info.Warnings, "Missing media: "+collection(r.kind)+"/"+r.name)
		}
	}
	root := p.XML.Root()
	if logo := root.SelectAttrValue("logo", ""); strings.HasPrefix(logo, "@") {
		add(reference{root, "logo", "@", "image", logo[1:]})
	}
	walk(root, func(e *etree.Element) {
		kind := e.SelectAttrValue("type", "")
		if kind == "voice" {
			kind = "audio"
		}
		if collection(kind) == "" {
			return
		}
		if e.Tag == "atom" && strings.HasPrefix(e.Text(), "@") {
			add(reference{e, "", "@", kind, e.Text()[1:]})
		}
		if e.Tag == "item" && strings.EqualFold(e.SelectAttrValue("isRef", "false"), "true") {
			add(reference{e, "", "", kind, e.Text()})
		}
	})
}

type Transform func(context.Context, Media, io.Reader) (filename string, warning string, err error)
type Progress func(Media, string, int64, string)

// Write publishes a complete archive only; the caller owns the temporary output path.
func (p *Package) Prepare(target string) error {
	if p.preparedFor != "" {
		if p.preparedFor != target {
			return fmt.Errorf("package already prepared for another output version")
		}
		return nil
	}
	if target != "source" && target != "5" {
		return fmt.Errorf("output_version must be 5 or source")
	}
	if target == "5" && p.Info.Version != "5" {
		if err := p.upgrade(); err != nil {
			return err
		}
		p.collectReferences()
	}
	plan, err := p.planUsage()
	if err != nil {
		return err
	}
	p.updateUsage(plan)
	p.usage = plan
	p.preparedFor = target
	return nil
}

func (p *Package) Write(ctx context.Context, out, target string, transform Transform, progress Progress) error {
	if err := p.Prepare(target); err != nil {
		return err
	}
	plan := p.usage
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	z := zip.NewWriter(f)
	defer z.Close()
	written := map[string][32]byte{}
	renames := map[string]string{}
	changed := plan.unusedCount > 0
	for _, src := range p.Archive.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		if src.FileInfo().IsDir() || src.Name == "content.xml" || src.Name == "[Content_Types].xml" || !plan.used[src.Name] {
			continue
		}
		kind := mediaKind(src.Name)
		if _, deferred := plan.markup[src.Name]; deferred {
			continue
		}
		if kind == "" || kind == "html" || plan.protected[src.Name] {
			if err := z.Copy(src); err != nil {
				return err
			}
			if kind != "" && progress != nil {
				progress(Media{src.Name, kind, int64(src.UncompressedSize64), strings.ToLower(path.Ext(src.Name)), len(p.refs[src.Name])}, src.Name, int64(src.UncompressedSize64), "")
			}
			continue
		}
		m := Media{src.Name, kind, int64(src.UncompressedSize64), strings.ToLower(path.Ext(src.Name)), len(p.refs[src.Name])}
		converted, warning := "", ""
		if transform != nil {
			r, err := src.Open()
			if err != nil {
				return err
			}
			var convErr error
			converted, warning, convErr = transform(ctx, m, r)
			r.Close()
			if convErr != nil {
				warning = convErr.Error()
				if converted != "" {
					os.Remove(converted)
					converted = ""
				}
			}
		}
		if ctx.Err() != nil {
			if converted != "" {
				os.Remove(converted)
			}
			return ctx.Err()
		}
		size, ext := m.Size, m.Extension
		openFinal := src.Open
		if converted != "" {
			st, err := os.Stat(converted)
			if err != nil {
				return err
			}
			if st.Size() < m.Size {
				size = st.Size()
				ext = strings.ToLower(path.Ext(converted))
				openFinal = func() (io.ReadCloser, error) { return os.Open(converted) }
				changed = true
			} else {
				os.Remove(converted)
				converted = ""
			}
		}
		newName, err := writeMD5Resource(z, path.Dir(src.Name), ext, openFinal, written)
		if converted != "" {
			os.Remove(converted)
		}
		if err != nil {
			return err
		}
		renames[src.Name] = newName
		if newName != src.Name {
			changed = true
		}
		for _, ref := range p.refs[src.Name] {
			name := strings.TrimPrefix(newName, collection(ref.kind)+"/")
			if ref.attr != "" {
				ref.element.CreateAttr(ref.attr, ref.prefix+name)
			} else {
				ref.element.SetText(ref.prefix + name)
			}
		}
		if progress != nil {
			progress(m, newName, size, warning)
		}
	}
	// HTML/CSS retain their names while static links follow renamed media.
	for _, src := range p.Archive.File {
		data, ok := plan.markup[src.Name]
		if !ok || !plan.used[src.Name] {
			continue
		}
		if !plan.protected[src.Name] {
			links := plan.links[src.Name]
			sort.Slice(links, func(i, j int) bool { return links[i].start > links[j].start })
			last := len(data)
			for _, link := range links {
				newName, ok := renames[link.target]
				if !ok || newName == link.target || link.end > last {
					continue
				}
				replacement := relativeAsset(src.Name, newName) + link.suffix
				if link.attribute {
					replacement = html.EscapeString(replacement)
				}
				data = append(append(append([]byte(nil), data[:link.start]...), []byte(replacement)...), data[link.end:]...)
				last = link.start
				changed = true
			}
		}
		w, err := z.Create(src.Name)
		if err != nil {
			return err
		}
		if _, err = w.Write(data); err != nil {
			return err
		}
		if mediaKind(src.Name) == "html" && progress != nil {
			progress(Media{src.Name, "html", int64(src.UncompressedSize64), path.Ext(src.Name), len(p.refs[src.Name])}, src.Name, int64(len(data)), "")
		}
	}
	if _, ok := p.files["[Content_Types].xml"]; ok {
		data, err := p.contentTypes(renames, plan)
		if err != nil {
			return err
		}
		w, err := z.Create("[Content_Types].xml")
		if err != nil {
			return err
		}
		if _, err = w.Write(data); err != nil {
			return err
		}
	}
	if changed {
		if hashes := p.XML.Root().SelectElement("files"); hashes != nil {
			p.XML.Root().RemoveChild(hashes)
		}
	}
	// SIPackages skips self-closing params but retains explicit empty values.
	p.XML.WriteSettings.CanonicalEndTags = true
	b, err := p.XML.WriteToBytes()
	if err != nil {
		return err
	}
	w, err := z.Create("content.xml")
	if err != nil {
		return err
	}
	if _, err = w.Write(b); err != nil {
		return err
	}
	if err = z.Close(); err != nil {
		return err
	}
	return f.Sync()
}

// MD5 is only a filename, never an integrity/security decision. Compare SHA-256
// too before coalescing entries, so distinct content is not lost to an MD5 collision.
func writeMD5Resource(z *zip.Writer, dir, ext string, open func() (io.ReadCloser, error), written map[string][32]byte) (string, error) {
	r, err := open()
	if err != nil {
		return "", err
	}
	nameHash, strongHash := md5.New(), sha256.New()
	_, err = io.Copy(io.MultiWriter(nameHash, strongHash), r)
	r.Close()
	if err != nil {
		return "", err
	}
	name := path.Join(dir, hex.EncodeToString(nameHash.Sum(nil))+ext)
	var digest [32]byte
	copy(digest[:], strongHash.Sum(nil))
	if existing, ok := written[name]; ok {
		if existing != digest {
			return "", fmt.Errorf("MD5 filename collision: %s", name)
		}
		return name, nil
	}
	w, err := z.Create(name)
	if err != nil {
		return "", err
	}
	r, err = open()
	if err != nil {
		return "", err
	}
	_, err = io.Copy(w, r)
	r.Close()
	if err != nil {
		return "", err
	}
	written[name] = digest
	return name, nil
}
