package siq

import (
	"archive/zip"
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmptyLegacyParametersSurviveSIPackagesReader(t *testing.T) {
	input := `<package version="4"><rounds><round><themes><theme><questions><question price="200"><type name="cat"><param name="cost">333</param></type><scenario><atom time="86401">text</atom></scenario></question></questions></theme></themes></round></rounds></package>`
	p, err := Open(fixture(t, input, nil), DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	out := filepath.Join(t.TempDir(), "converted.siq")
	if err := p.Write(context.Background(), out, "5", nil, nil); err != nil {
		t.Fatal(err)
	}
	z, err := zip.OpenReader(out)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	r, _ := z.Open("content.xml")
	data, _ := io.ReadAll(r)
	r.Close()
	// XmlReader.IsEmptyElement must be false: the official reader skips <param/>.
	if !strings.Contains(string(data), `<param name="theme"></param>`) {
		t.Fatal("empty theme will be dropped by SIPackages")
	}
	root := outputXML(t, out)
	if root.FindElement(".//item").SelectAttrValue("duration", "") != "1.00:00:01" {
		t.Fatal("TimeSpan days were lost")
	}
}

func TestXMLComplexityAndArchiveLimits(t *testing.T) {
	deep := `<package version="5">` + strings.Repeat("<x>", 130) + strings.Repeat("</x>", 130) + `</package>`
	if p, err := Open(fixture(t, deep, nil), DefaultLimits()); err == nil {
		p.Close()
		t.Fatal("deep XML accepted")
	}
	limits := DefaultLimits()
	limits.FileBytes = 32
	if p, err := Open(fixture(t, `<package version="5"/>`, map[string]string{"extra": strings.Repeat("a", 33)}), limits); err == nil {
		p.Close()
		t.Fatal("large entry accepted")
	}
}
