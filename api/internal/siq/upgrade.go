package siq

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/beevik/etree"
)

// This conversion follows SIPackages.Question.Upgrade at upstream commit
// eaf15e4cbaa414c827c48bc751532ba58e2cac73. See THIRD_PARTY_NOTICES.md.
func (p *Package) upgrade() error {
	root := p.XML.Root()
	var questions []*etree.Element
	walk(root, func(e *etree.Element) {
		if e.Tag == "question" {
			questions = append(questions, e)
		}
	})
	for _, q := range questions {
		if err := upgradeQuestion(q); err != nil {
			return err
		}
	}
	// Old packages keep referenced author/source records in separate XML files.
	for _, kind := range []string{"authors", "sources"} {
		name := "Texts/" + kind + ".xml"
		if _, ok := p.files[name]; !ok {
			continue
		}
		b, err := p.read(name, p.limits.XMLBytes)
		if err != nil {
			return err
		}
		d, err := parseXML(b)
		if err != nil {
			return err
		}
		if d.Root() == nil {
			continue
		}
		g := root.SelectElement("global")
		if g == nil {
			g = root.CreateElement("global")
		}
		dest := g.SelectElement(kind)
		if dest == nil {
			dest = g.CreateElement(kind)
		}
		for _, e := range d.Root().ChildElements() {
			dest.AddChild(e.Copy())
		}
	}
	oldNS := root.NamespaceURI()
	walk(root, func(e *etree.Element) {
		for i := range e.Attr {
			a := &e.Attr[i]
			if (a.Key == "xmlns" || a.Space == "xmlns") && a.Value == oldNS {
				a.Value = Namespace
			}
		}
	})
	if root.Space == "" {
		root.CreateAttr("xmlns", Namespace)
	}
	root.CreateAttr("version", LatestVersion)
	return nil
}

func upgradeQuestion(q *etree.Element) error {
	if q.SelectElement("params") != nil || q.SelectElement("script") != nil {
		return fmt.Errorf("legacy question contains modern params/script; refusing ambiguous conversion")
	}
	t := q.SelectElement("type")
	kind := "simple"
	values := map[string]string{}
	if t != nil {
		kind = t.SelectAttrValue("name", "simple")
		for _, p := range t.SelectElements("param") {
			values[p.SelectAttrValue("name", "")] = p.Text()
		}
	}
	params := etree.NewElement("params")
	params.Space = q.Space
	param := func(name, typ, value string) *etree.Element {
		e := params.CreateElement("param")
		e.Space = q.Space
		e.CreateAttr("name", name)
		if typ != "" {
			e.CreateAttr("type", typ)
		}
		if value != "" {
			e.SetText(value)
		}
		return e
	}
	newType := kind
	noQuestion := false
	if q.SelectAttrValue("price", "") == "-1" {
		newType = ""
		noQuestion = true
	} else {
		switch kind {
		case "simple":
			newType = ""
		case "auction":
			newType = "stake"
		case "sponsored":
			newType = "noRisk"
		case "cat", "bagcat":
			newType = "secret"
			knows := values["knows"]
			if kind == "bagcat" && knows == "before" {
				newType = "secretPublicPrice"
			}
			if kind == "bagcat" && knows == "never" {
				newType = "secretNoQuestion"
				noQuestion = true
			}
			if !noQuestion {
				param("theme", "", values["theme"])
			}
			price := param("price", "numberSet", "")
			n := price.CreateElement("numberSet")
			n.Space = q.Space
			min, max, step := numberSet(values["cost"])
			n.CreateAttr("minimum", strconv.Itoa(min))
			n.CreateAttr("maximum", strconv.Itoa(max))
			n.CreateAttr("step", strconv.Itoa(step))
			mode := "exceptCurrent"
			if kind == "bagcat" && values["self"] == "true" {
				mode = "any"
			}
			param("selectionMode", "", mode)
		default:
			if t != nil {
				for _, e := range t.SelectElements("param") {
					param(e.SelectAttrValue("name", ""), "", e.Text())
				}
			}
		}
	}
	q.RemoveAttr("type")
	if newType != "" {
		q.CreateAttr("type", newType)
	}
	scenario := q.SelectElement("scenario")
	if !noQuestion {
		current := param("question", "content", "")
		var answer *etree.Element
		if scenario != nil {
			for _, atom := range scenario.ChildElements() {
				if atom.Tag != "atom" {
					return fmt.Errorf("unsupported legacy scenario element %s", atom.Tag)
				}
				typ := atom.SelectAttrValue("type", "text")
				if typ == "marker" {
					if answer == nil {
						answer = etree.NewElement("param")
						answer.Space = q.Space
						answer.CreateAttr("name", "answer")
						answer.CreateAttr("type", "content")
						current = answer
					}
					continue
				}
				e := current.CreateElement("item")
				e.Space = q.Space
				switch typ {
				case "oral":
					typ = "text"
					e.CreateAttr("placement", "replic")
				case "voice":
					typ = "audio"
					e.CreateAttr("placement", "background")
				}
				if typ != "text" {
					e.CreateAttr("type", typ)
				}
				value := atom.Text()
				if strings.HasPrefix(value, "@") {
					e.CreateAttr("isRef", "true")
					value = value[1:]
				}
				e.SetText(value)
				time, err := strconv.Atoi(atom.SelectAttrValue("time", "0"))
				if err != nil || time < -1 {
					return fmt.Errorf("invalid legacy atom time")
				}
				if time == -1 {
					e.CreateAttr("waitForFinish", "false")
				} else if time > 0 {
					duration := fmt.Sprintf("%02d:%02d:%02d", time/3600%24, time/60%60, time%60)
					if time >= 86400 {
						duration = strconv.Itoa(time/86400) + "." + duration
					}
					e.CreateAttr("duration", duration)
				}
			}
		}
		if answer != nil && len(answer.ChildElements()) > 0 {
			params.AddChild(answer)
		}
	}
	if t != nil {
		q.RemoveChild(t)
	}
	if scenario != nil {
		q.RemoveChild(scenario)
	}
	if len(params.ChildElements()) > 0 {
		q.AddChild(params)
	}
	if q.SelectElement("right") == nil {
		r := q.CreateElement("right")
		r.Space = q.Space
		r.CreateElement("answer").Space = q.Space
	}
	return nil
}

var numberSetPattern = regexp.MustCompile(`\[(\d+);(\d+)\](?:/(\d+))?`)

func numberSet(s string) (int, int, int) {
	if n, err := strconv.Atoi(s); err == nil {
		return n, n, 0
	}
	m := numberSetPattern.FindStringSubmatch(s)
	if m == nil {
		return 0, 0, 0
	}
	a, _ := strconv.Atoi(m[1])
	b, _ := strconv.Atoi(m[2])
	step := b - a
	if m[3] != "" {
		step, _ = strconv.Atoi(m[3])
	}
	return a, b, step
}
