package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/cuvva/cuvva-public-go/lib/cher"
	"golang.org/x/sync/errgroup"
)

func downloadAssets(assets []Asset) error {
	g, _ := errgroup.WithContext(context.Background())

	for _, a := range assets {
		asset := a

		g.Go(func() error {
			fmt.Println("⏬ Downloading", asset.Name)

			res, err := http.Get(asset.BrowserDownloadURL)
			if err != nil {
				return err
			}
			defer res.Body.Close()

			if res.StatusCode != 200 {
				return cher.New("bad_hit_for_url", cher.M{"asset": asset.Name})
			}

			out, err := os.Create(asset.Name)
			if err != nil {
				return err
			}
			defer out.Close()

			if _, err = io.Copy(out, res.Body); err != nil {
				return err
			}

			fmt.Println("✅ Downloaded", asset.Name)

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	return nil
}

func cleanupAssets(assets []Asset) error {
	fmt.Println("🧽 Cleaning up!")

	for _, a := range assets {
		asset := a

		os.Remove(asset.Name)
	}

	return nil
}

func moveAssets(prefix, folderPath string, assets []Asset) error {
	fmt.Println("➡️ ", "Moving files to", folderPath)

	for _, a := range assets {
		asset := a

		fileName := strings.TrimPrefix(asset.Name, fmt.Sprintf("%s-", prefix))
		filePath := path.Join(folderPath, fileName)

		if err := copy(asset.Name, filePath); err != nil {
			return err
		}

		if runtime.GOOS != "windows" {
			if err := os.Chown(filePath, os.Getuid(), os.Getgid()); err != nil {
				return err
			}

			if err := os.Chmod(filePath, 0774); err != nil {
				return err
			}
		}
	}

	return nil
}

func copy(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}
