package siq

import (
	"archive/zip"
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/beevik/etree"
)

func md5Name(value, ext string) string { return fmt.Sprintf("%x%s", md5.Sum([]byte(value)), ext) }

func fixture(t *testing.T, xml string, files map[string]string) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "test.siq")
	f, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	w, _ := z.Create("content.xml")
	_, _ = io.WriteString(w, xml)
	for name, b := range files {
		w, _ = z.Create(name)
		_, _ = io.WriteString(w, b)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return name
}
func outputXML(t *testing.T, name string) *etree.Element {
	t.Helper()
	z, err := zip.OpenReader(name)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	r, err := z.Open("content.xml")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	b, _ := io.ReadAll(r)
	d, err := parseXML(b)
	if err != nil {
		t.Fatal(err)
	}
	return d.Root()
}
func TestUpgradeVersionsAndQuestionSemantics(t *testing.T) {
	for _, version := range []string{"3", "3.1", "4"} {
		t.Run(version, func(t *testing.T) {
			xml := `<package xmlns="http://vladimirkhil.com/ygpackage3.0.xsd" name="Quiz" version="` + version + `"><rounds><round name="Round"><themes><theme name="Theme"><questions><question price="100"><type name="bagcat"><param name="theme">Secret</param><param name="cost">[100;500]/100</param><param name="self">true</param><param name="knows">before</param></type><scenario><atom>Hello &amp; world</atom><atom type="voice" time="-1">@sound.mp3</atom><atom type="marker"/><atom type="image" time="5">@picture.png</atom></scenario><right><answer>Yes</answer></right><info><comments>Keep me</comments></info></question></questions></theme></themes></round></rounds></package>`
			p, err := Open(fixture(t, xml, map[string]string{"Audio/sound.mp3": "audio", "Images/picture.png": "image", "Texts/authors.xml": `<authors><author id="a"><firstName>A</firstName></author></authors>`}), DefaultLimits())
			if err != nil {
				t.Fatal(err)
			}
			defer p.Close()
			if p.Info.Questions != 1 || len(p.Info.Media) != 2 {
				t.Fatalf("analysis: %+v", p.Info)
			}
			out := filepath.Join(t.TempDir(), "out.siq")
			if err = p.Write(context.Background(), out, "5", nil, nil); err != nil {
				t.Fatal(err)
			}
			r := outputXML(t, out)
			if r.SelectAttrValue("version", "") != "5" || r.NamespaceURI() != Namespace {
				t.Fatal("wrong version/namespace")
			}
			q := r.FindElement(".//question")
			if q.SelectAttrValue("type", "") != "secretPublicPrice" {
				t.Fatal("question type lost")
			}
			if q.FindElement("params/param[@name='price']/numberSet").SelectAttrValue("step", "") != "100" {
				t.Fatal("price lost")
			}
			voice := q.FindElement("params/param[@name='question']/item[@type='audio']")
			if voice.Text() != md5Name("audio", ".mp3") || voice.SelectAttrValue("isRef", "") != "true" || voice.SelectAttrValue("waitForFinish", "") != "false" || voice.SelectAttrValue("placement", "") != "background" {
				t.Fatalf("voice: %+v", voice)
			}
			answer := q.FindElement("params/param[@name='answer']/item")
			if answer.Text() != md5Name("image", ".png") || answer.SelectAttrValue("duration", "") != "00:00:05" {
				t.Fatal("answer timing lost")
			}
			if r.FindElement("global/authors/author") == nil || q.FindElement("info/comments").Text() != "Keep me" {
				t.Fatal("metadata lost")
			}
		})
	}
}
func TestSharedMediaExternalURLsAndCleanup(t *testing.T) {
	xml := `<package version="5" name="Test"><files><file name="Images/a.png" hash="obsolete"/></files><extension>custom</extension><rounds><round><themes><theme><questions><question><params><param name="question" type="content"><item type="image" isRef="true">a.png</item><item type="image" isRef="true">a.png</item><item type="image">https://example.com/a.png</item></param></params></question></questions></theme></themes></round></rounds></package>`
	p, err := Open(fixture(t, xml, map[string]string{"Images/a.png": strings.Repeat("a", 100), "extra.bin": "untouched", "Images/unreferenced.png": "keep"}), DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	calls := 0
	transform := func(ctx context.Context, m Media, r io.Reader) (string, string, error) {
		calls++
		name := filepath.Join(t.TempDir(), "out.webp")
		os.WriteFile(name, []byte("small"), 0600)
		return name, "", nil
	}
	out := filepath.Join(t.TempDir(), "out.siq")
	if err = p.Write(context.Background(), out, "5", transform, nil); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("processed %d times", calls)
	}
	r := outputXML(t, out)
	items := r.FindElements(".//item")
	if items[0].Text() != items[1].Text() || !strings.HasSuffix(items[0].Text(), ".webp") || items[2].Text() != "https://example.com/a.png" {
		t.Fatal("broken references")
	}
	if r.SelectElement("files") != nil || r.SelectElement("extension").Text() != "custom" {
		t.Fatal("metadata handling")
	}
	z, _ := zip.OpenReader(out)
	defer z.Close()
	for _, name := range []string{"extra.bin", "Images/unreferenced.png"} {
		f, err := z.Open(name)
		if err == nil {
			f.Close()
			t.Fatalf("unused file retained: %s", name)
		}
	}
}
func TestSourceVersionAndEscapedNames(t *testing.T) {
	xml := `<package version="3"><rounds><round><themes><theme><questions><question><scenario><atom type="image">@тест a%.png</atom></scenario></question></questions></theme></themes></round></rounds></package>`
	p, err := Open(fixture(t, xml, map[string]string{"Images/%D1%82%D0%B5%D1%81%D1%82%20a%25.png": "x"}), DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if len(p.Info.Media) != 1 {
		t.Fatal(p.Info.Warnings)
	}
	out := filepath.Join(t.TempDir(), "out.siq")
	if err = p.Write(context.Background(), out, "source", nil, nil); err != nil {
		t.Fatal(err)
	}
	if outputXML(t, out).SelectAttrValue("version", "") != "3" {
		t.Fatal("source version changed")
	}
}
func TestRejectUnsafeAndUnknownPackages(t *testing.T) {
	for name, xml := range map[string]string{"future": `<package version="6"/>`, "missing": `<package/>`, "dtd": `<!DOCTYPE package [<!ENTITY x "x">]><package version="5"/>`} {
		t.Run(name, func(t *testing.T) {
			if p, err := Open(fixture(t, xml, nil), DefaultLimits()); err == nil {
				p.Close()
				t.Fatal("accepted invalid package")
			}
		})
	}
	for _, name := range []string{"../escape", "/absolute", "C:/file", "Images/../../file", "Images\\file"} {
		t.Run(name, func(t *testing.T) {
			if p, err := Open(fixture(t, `<package version="5"/>`, map[string]string{name: "x"}), DefaultLimits()); err == nil {
				p.Close()
				t.Fatal("accepted unsafe entry")
			}
		})
	}
}
func TestKeepLargerOutputAndCancellation(t *testing.T) {
	xml := `<package version="5" logo="@a.png"/>`
	p, err := Open(fixture(t, xml, map[string]string{"Images/a.png": "x"}), DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	out := filepath.Join(t.TempDir(), "out.siq")
	err = p.Write(context.Background(), out, "5", func(context.Context, Media, io.Reader) (string, string, error) {
		name := filepath.Join(t.TempDir(), "large.webp")
		os.WriteFile(name, []byte("larger"), 0600)
		return name, "", nil
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if outputXML(t, out).SelectAttrValue("logo", "") != "@"+md5Name("x", ".png") {
		t.Fatal("kept larger output")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err = p.Write(ctx, out, "5", nil, nil); err != context.Canceled {
		t.Fatal(err)
	}
}
