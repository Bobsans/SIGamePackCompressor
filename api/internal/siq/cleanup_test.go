package siq

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

func readArchive(t *testing.T, filename string) map[string][]byte {
	t.Helper()
	z, err := zip.OpenReader(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	files := map[string][]byte{}
	for _, f := range z.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		files[f.Name] = b
	}
	return files
}

func TestMD5NamesDeduplicateAndKeepOnlyReferencedResources(t *testing.T) {
	xml := `<package version="5" logo="@cover.png"><rounds><round><themes><theme><questions><question><params><param name="question" type="content"><item type="image" isRef="true">short.webp</item><item type="image" isRef="true">` + strings.Repeat("a", 64) + `.webp</item><item type="audio" isRef="true">voice.mp3</item><item type="image">https://example.com/unused.png</item></param><param name="answer" type="content"><item type="image" isRef="true">answer.png</item></param></params><script><step><params><param type="content"><item type="video" isRef="true">clip.mp4</item></param></params></step></script></question></questions></theme></themes></round></rounds></package>`
	files := map[string]string{"Images/cover.png": "cover", "Images/short.webp": "same image", "Images/" + strings.Repeat("a", 64) + ".webp": "same image", "Images/answer.png": "answer", "Audio/voice.mp3": "audio", "Video/clip.mp4": "video", "Images/unused.png": "unused", "Video/unused.mp4": "unused video", "random.bin": "trash", "quality.marker": ""}
	p, err := Open(fixture(t, xml, files), DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if p.Info.UnusedFiles != 3 {
		t.Fatalf("unused count: %d", p.Info.UnusedFiles)
	}
	out := filepath.Join(t.TempDir(), "out.siq")
	if err = p.Write(context.Background(), out, "5", nil, nil); err != nil {
		t.Fatal(err)
	}
	result := readArchive(t, out)
	if len(result) != 7 {
		t.Fatalf("unexpected output entries: %v", result)
	}
	for name, data := range result {
		if name == "content.xml" || name == "quality.marker" {
			continue
		}
		want := fmt.Sprintf("%x%s", md5.Sum(data), path.Ext(name))
		if path.Base(name) != want {
			t.Fatalf("non-MD5 name: %s", name)
		}
	}
	r := outputXML(t, out)
	items := r.FindElements(".//item")
	if items[0].Text() != items[1].Text() {
		t.Fatal("duplicate resources not coalesced")
	}
	reopened, err := Open(out, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if len(reopened.Info.Warnings) != 0 {
		t.Fatal(reopened.Info.Warnings)
	}
}

func TestStaticHTMLDependenciesFollowRenames(t *testing.T) {
	xml := `<package version="5"><question><params><param type="content"><item type="html" isRef="true">page.html</item></param></params></question></package>`
	files := map[string]string{"Html/page.html": `<html><!-- <script src="removed.js"></script><img src="../Images/unused.png"> --><link href="style.css"><img src="../Images/pic%20one.png?x=1&amp;y=2#top"></html>`, "Html/style.css": `body{background:url('../Images/background.png')}`, "Images/pic one.png": "picture", "Images/background.png": "background", "Images/unused.png": "trash", "Html/unused.html": `<script>unused()</script>`}
	p, err := Open(fixture(t, xml, files), DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if p.Info.UnusedFiles != 2 {
		t.Fatal(p.Info)
	}
	out := filepath.Join(t.TempDir(), "out.siq")
	if err = p.Write(context.Background(), out, "5", nil, nil); err != nil {
		t.Fatal(err)
	}
	result := readArchive(t, out)
	if !strings.Contains(string(result["Html/page.html"]), "../Images/"+md5Name("picture", ".png")+"?x=1&amp;y=2#top") {
		t.Fatal(string(result["Html/page.html"]))
	}
	if !strings.Contains(string(result["Html/style.css"]), "../Images/"+md5Name("background", ".png")) {
		t.Fatal(string(result["Html/style.css"]))
	}
	if _, exists := result["Images/unused.png"]; exists {
		t.Fatal("unused image retained")
	}
}

func TestDynamicHTMLDoesNotLoseRuntimeAssets(t *testing.T) {
	xml := `<package version="5"><item type="html" isRef="true">game.html</item></package>`
	files := map[string]string{"Html/game.html": `<script>const p='../Images/'+id+'.png'</script>`, "Images/1.png": "runtime asset"}
	p, err := Open(fixture(t, xml, files), DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if p.Info.UnusedFiles != 0 || len(p.Info.Warnings) == 0 {
		t.Fatal("dynamic asset policy is not observable")
	}
	out := filepath.Join(t.TempDir(), "out.siq")
	if err = p.Write(context.Background(), out, "5", nil, nil); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(readArchive(t, out)["Images/1.png"], []byte("runtime asset")) {
		t.Fatal("runtime dependency lost")
	}
}

func TestMD5RenamesFallbackOriginalAndUpdatedContent(t *testing.T) {
	xml := `<package version="5"><item type="image" isRef="true">one.webp</item><item type="image" isRef="true">two.png</item></package>`
	p, err := Open(fixture(t, xml, map[string]string{"Images/one.webp": "old long content", "Images/two.png": "original"}), DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	out := filepath.Join(t.TempDir(), "out.siq")
	err = p.Write(context.Background(), out, "5", func(ctx context.Context, m Media, r io.Reader) (string, string, error) {
		if strings.HasSuffix(m.Name, "two.png") {
			return "", "", fmt.Errorf("decoder failed")
		}
		f := filepath.Join(t.TempDir(), "new.webp")
		os.WriteFile(f, []byte("small"), 0600)
		return f, "", nil
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	files := readArchive(t, out)
	for _, name := range []string{"Images/" + md5Name("small", ".webp"), "Images/" + md5Name("original", ".png")} {
		if _, ok := files[name]; !ok {
			t.Fatalf("missing %s", name)
		}
	}
}

func TestMD5CollisionDoesNotDiscardDifferentBytes(t *testing.T) {
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	name := "Images/" + md5Name("data", ".png")
	_, err := writeMD5Resource(z, "Images", ".png", func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("data")), nil }, map[string][32]byte{name: {1}})
	if err == nil {
		t.Fatal("distinct bytes silently merged")
	}
	z.Close()
}
