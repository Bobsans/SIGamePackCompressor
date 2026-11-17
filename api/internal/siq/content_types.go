package siq

import (
	"net/url"
	"path"
	"strings"
)

func (p *Package) contentTypes(renames map[string]string, plan *usagePlan) ([]byte, error) {
	b, err := p.read("[Content_Types].xml", p.limits.XMLBytes)
	if err != nil {
		return nil, err
	}
	d, err := parseXML(b)
	if err != nil {
		return nil, err
	}
	root := d.Root()
	if root == nil {
		return b, nil
	}
	mimes := map[string]string{".webp": "image/webp", ".avif": "image/avif", ".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif", ".mp3": "audio/mpeg", ".opus": "audio/ogg", ".mp4": "video/mp4"}
	defined := map[string]bool{}
	for _, e := range root.ChildElements() {
		if e.Tag == "Default" {
			defined[strings.ToLower(e.SelectAttrValue("Extension", ""))] = true
		}
		if e.Tag == "Override" {
			name := strings.TrimPrefix(e.SelectAttrValue("PartName", ""), "/")
			decoded, decodeErr := url.PathUnescape(name)
			if decodeErr == nil {
				if _, exists := p.files[decoded]; exists {
					name = decoded
				}
			}
			if _, exists := p.files[name]; exists && !plan.used[name] {
				root.RemoveChild(e)
				continue
			}
			if replacement, ok := renames[name]; ok {
				e.CreateAttr("PartName", "/"+(&url.URL{Path: replacement}).EscapedPath())
				if mime := mimes[path.Ext(replacement)]; mime != "" {
					e.CreateAttr("ContentType", mime)
				}
			}
		}
	}
	for _, name := range renames {
		ext := path.Ext(name)
		mime := mimes[ext]
		if mime == "" || defined[strings.TrimPrefix(ext, ".")] {
			continue
		}
		e := root.CreateElement("Default")
		e.Space = root.Space
		e.CreateAttr("Extension", strings.TrimPrefix(ext, "."))
		e.CreateAttr("ContentType", mime)
		defined[strings.TrimPrefix(ext, ".")] = true
	}
	return d.WriteToBytes()
}
