package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

type TokenResponse struct {
	Token       string `json:"token"`
	AccessToken string `json:"access_token"`
	Expires     int    `json:"expires_in"`
	IssuedAt    string `json:"issued_at"`
}

type ManiFest struct {
	Name     string     `json:"name"`
	Tag      string     `json:"tag"`
	FSLayers []fsLayers `json:"fsLayers"`
}

type fsLayers struct {
	BlobSum string `json:"blobSum"`
}

func getToken(repo, image string) (*TokenResponse, error) {
	url := fmt.Sprintf("https://auth.docker.io/token?service=registry.docker.io&scope=repository:%s/%s:pull", repo, image)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var token TokenResponse

	err = json.NewDecoder(resp.Body).Decode(&token)
	if err != nil {
		return nil, err
	}

	return &token, nil
}

func getManifest(repo, image, tag string, token string) (*ManiFest, error) {
	url := fmt.Sprintf("https://registry.hub.docker.com/v2/%s/%s/manifests/%s", repo, image, tag)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Add("Accept", "application/vnd.docker.distribution.manifest.list.v1+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var manifest ManiFest

	err = json.NewDecoder(resp.Body).Decode(&manifest)
	if err != nil {
		return nil, err
	}

	return &manifest, nil
}

func downloadBlob(image, blobSum, token, tmpDir string) error {
	url := fmt.Sprintf("https://registry-1.docker.io/v2/library/%s/blobs/%s", image, blobSum)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	out, err := os.Create("/tmp/layer")
	if err != nil {
		return err
	}

	defer out.Close()

	_, err = out.ReadFrom(resp.Body)
	if err != nil {
		return err
	}

	err = exec.Command("tar", "xf", "/tmp/layer", "-C", tmpDir).Run()
	if err != nil {
		return err
	}

	return os.RemoveAll("/tmp/layer")
}

func main() {
	img := os.Args[2]
	split := strings.Split(img, ":")
	repo := "library"
	image := split[0]
	tag := "latest"
	if len(split) == 2 {
		tag = split[1]
	}

	command := os.Args[3]
	args := os.Args[4:len(os.Args)]
	tmpDir := "/tmp/mydocker"

	_ = os.Mkdir(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	err := exec.Command("mkdir", "-p", filepath.Join(tmpDir, filepath.Dir(command))).Run()
	if err != nil {
		fmt.Printf("Err: %v", err)
		os.Exit(1)
	}

	token, err := getToken(repo, image)
	if err != nil {
		fmt.Printf("Err: %v", err)
		os.Exit(1)
	}

	manifest, err := getManifest(repo, image, tag, token.Token)
	if err != nil {
		fmt.Printf("Err: %v", err)
		os.Exit(1)
	}

	for _, layer := range manifest.FSLayers {
		if err = downloadBlob(image, layer.BlobSum, token.Token, tmpDir); err != nil {
			fmt.Printf("Err: %v", err)
			os.Exit(1)
		}
	}

	cmd := exec.Command(command, args...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = exec.Command("cp", command, filepath.Join(tmpDir, command)).Run()
	if err != nil {
		fmt.Printf("Err: %v", err)
		os.Exit(1)
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Chroot:     tmpDir,
		Cloneflags: syscall.CLONE_NEWPID,
	}

	err = cmd.Run()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			os.Exit(exitError.ExitCode())
		} else {
			fmt.Printf("Err: %v", err)
			os.Exit(1)
		}
	}
}
