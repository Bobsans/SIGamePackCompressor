package siq

import (
	"bytes"
	"fmt"
	"html"
	"net/url"
	"path"
	"regexp"
	"strings"
)

type assetLink struct {
	start, end     int
	target, suffix string
	attribute      bool
}
type usagePlan struct {
	used        map[string]bool
	protected   map[string]bool
	links       map[string][]assetLink
	markup      map[string][]byte
	unusedCount int
	unusedSize  int64
}

// Static local links can be rewritten safely. Executable HTML, base URLs and
// compound srcsets need runtime knowledge, so retain their possible assets.
var dynamicMarkup = regexp.MustCompile(`(?is)<script\b|<base\b|\bsrcdoc\s*=|\bsrcset\s*=|\bon\w+\s*=|javascript:|\bimport\s*\(|\bimage-set\s*\(`)
var assetPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?is)\b(?:src|href|poster|data|background|target)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`),
	regexp.MustCompile(`(?is)url\(\s*(?:"([^"]*)"|'([^']*)'|([^\s)]+))\s*\)`),
	regexp.MustCompile(`(?is)@import\s+(?:"([^"]*)"|'([^']*)')`),
}
var markupComments = regexp.MustCompile(`(?s)<!--.*?-->|/\*.*?\*/`)

func coreEntry(name string) bool {
	switch name {
	case "content.xml", "quality.marker", "Texts/authors.xml", "Texts/sources.xml", "[Content_Types].xml":
		return true
	}
	return strings.HasPrefix(name, "_rels/") || strings.Contains(name, "/_rels/")
}

func mediaKind(name string) string {
	for kind, dir := range map[string]string{"image": "Images/", "audio": "Audio/", "video": "Video/", "html": "Html/"} {
		if strings.HasPrefix(name, dir) {
			if kind != "html" {
				return kind
			}
			if ext := strings.ToLower(path.Ext(name)); ext == ".html" || ext == ".htm" {
				return "html"
			}
		}
	}
	switch strings.ToLower(path.Ext(name)) {
	case ".jpg", ".jpe", ".jpeg", ".png", ".gif", ".webp", ".avif", ".bmp", ".tif", ".tiff":
		return "image"
	case ".mp3", ".opus", ".wav", ".ogg", ".flac", ".aac":
		return "audio"
	case ".mp4", ".mkv", ".webm", ".avi", ".mov", ".wmv":
		return "video"
	}
	return ""
}

func (p *Package) localAsset(owner, value string) (string, string) {
	owner = linkOwner(owner)
	value = html.UnescapeString(strings.TrimSpace(value))
	u, err := url.Parse(value)
	if err != nil || u.IsAbs() || u.Host != "" || u.Path == "" {
		return "", ""
	}
	name := path.Clean(path.Join(path.Dir(owner), u.Path))
	if strings.HasPrefix(u.Path, "/") {
		name = strings.TrimPrefix(path.Clean(u.Path), "/")
	}
	if !safeName(name) {
		return "", ""
	}
	suffix := ""
	if u.RawQuery != "" {
		suffix += "?" + u.RawQuery
	}
	if u.Fragment != "" {
		suffix += "#" + u.EscapedFragment()
	}
	if _, ok := p.files[name]; ok {
		return name, suffix
	}
	for candidate := range p.files {
		decoded, err := url.PathUnescape(candidate)
		if err == nil && decoded == name {
			return candidate, suffix
		}
	}
	return "", ""
}

func (p *Package) planUsage() (*usagePlan, error) {
	plan := &usagePlan{used: map[string]bool{}, protected: map[string]bool{}, links: map[string][]assetLink{}, markup: map[string][]byte{}}
	queue := []string{}
	mark := func(name string) {
		if !plan.used[name] {
			plan.used[name] = true
			queue = append(queue, name)
		}
	}
	for name := range p.refs {
		mark(name)
	}
	for name := range p.files {
		if coreEntry(name) {
			mark(name)
		}
	}
	conservative := false
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		ext := strings.ToLower(path.Ext(name))
		if ext == ".js" || ext == ".mjs" {
			conservative = true
			continue
		}
		if ext != ".html" && ext != ".htm" && ext != ".css" && ext != ".svg" && ext != ".rels" {
			continue
		}
		data, err := p.read(name, p.limits.XMLBytes)
		if err != nil {
			return nil, fmt.Errorf("inspect embedded asset %s: %w", name, err)
		}
		plan.markup[name] = data
		scan := bytes.Clone(data)
		for _, span := range markupComments.FindAllIndex(scan, -1) {
			for i := span[0]; i < span[1]; i++ {
				scan[i] = ' '
			}
		}
		if dynamicMarkup.Match(scan) {
			conservative = true
		}
		for patternIndex, pattern := range assetPatterns {
			for _, match := range pattern.FindAllSubmatchIndex(scan, -1) {
				for i := 2; i < len(match); i += 2 {
					if match[i] < 0 {
						continue
					}
					start, end := match[i], match[i+1]
					target, suffix := p.localAsset(name, string(data[start:end]))
					if target != "" {
						plan.links[name] = append(plan.links[name], assetLink{start, end, target, suffix, patternIndex == 0})
						mark(target)
					}
					break
				}
			}
		}
	}
	if conservative {
		// Scripts can construct paths dynamically. Do not guess which files they use.
		for name := range p.files {
			plan.used[name] = true
			plan.protected[name] = true
		}
		warning := "Dynamic HTML references detected; related archive resources are retained with original names for compatibility"
		found := false
		for _, w := range p.Info.Warnings {
			if w == warning {
				found = true
			}
		}
		if !found {
			p.Info.Warnings = append(p.Info.Warnings, warning)
		}
	}
	for name, file := range p.files {
		if !plan.used[name] {
			plan.unusedCount++
			plan.unusedSize += int64(file.UncompressedSize64)
		}
	}
	return plan, nil
}

func relativeAsset(owner, target string) string {
	owner = linkOwner(owner)
	from := strings.Split(path.Dir(owner), "/")
	if len(from) == 1 && from[0] == "." {
		from = nil
	}
	to := strings.Split(target, "/")
	for len(from) > 0 && len(to) > 0 && from[0] == to[0] {
		from = from[1:]
		to = to[1:]
	}
	parts := make([]string, 0, len(from)+len(to))
	for range from {
		parts = append(parts, "..")
	}
	parts = append(parts, to...)
	return (&url.URL{Path: strings.Join(parts, "/")}).EscapedPath()
}

func linkOwner(name string) string {
	if name == "_rels/.rels" {
		return "package"
	}
	if strings.Contains(name, "/_rels/") && strings.HasSuffix(name, ".rels") {
		return strings.TrimSuffix(strings.Replace(name, "/_rels/", "/", 1), ".rels")
	}
	return name
}
