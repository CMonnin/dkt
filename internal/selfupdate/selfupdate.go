// Package selfupdate replaces the running binary with the latest GitHub release.
package selfupdate

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/CMonnin/dwkt/internal/app"
)

type release struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// Run downloads the release archive matching GoReleaser's naming
// (<name>_<version>_<os>_<arch>.tar.gz) and atomically swaps the binary.
func Run(out io.Writer) error {
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Get("https://api.github.com/repos/" + app.GitHubRepo + "/releases/latest")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("latest release: %s", resp.Status)
	}
	var rel release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return err
	}
	if rel.TagName == app.Version || "v"+app.Version == rel.TagName {
		fmt.Fprintf(out, "already at %s\n", rel.TagName)
		return nil
	}
	suffix := "_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"
	var url string
	for _, a := range rel.Assets {
		if strings.HasSuffix(a.Name, suffix) {
			url = a.URL
		}
	}
	if url == "" {
		return fmt.Errorf("release %s has no %s asset", rel.TagName, suffix)
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return err
	}
	bin, err := client.Get(url)
	if err != nil {
		return err
	}
	defer bin.Body.Close()
	tmp := exe + ".new"
	if err := extract(bin.Body, tmp); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, exe); err != nil {
		os.Remove(tmp)
		return err
	}
	fmt.Fprintf(out, "updated %s → %s\n", app.Version, rel.TagName)
	return nil
}

func extract(r io.Reader, dst string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("archive has no %s binary", app.Name)
		}
		if err != nil {
			return err
		}
		if filepath.Base(h.Name) != app.Name || h.Typeflag != tar.TypeReg {
			continue
		}
		f, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			return err
		}
		return f.Close()
	}
}
