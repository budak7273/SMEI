package ue

import (
	"context"
	"fmt"
	"github.com/google/go-github/v42/github"
	"golang.org/x/oauth2"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
)

const orgName = "SatisfactoryModdingUE"
const repoName = "UnrealEngine"
const installerName = "UnrealEngine-CSS-Editor-Win64.exe"

func Install(githubToken, targetPath string, local bool) error {
	ctx := context.Background()
	client := makeGithubClient(ctx, githubToken)

	err := ensureGithubAccess(ctx, client, githubToken)
	if err != nil {
		return fmt.Errorf("could not ensure GitHub access: %v", err)
	}

	assetsToDownload, err := getAssetsToDownload(ctx, client)
	if err != nil {
		return fmt.Errorf("could not get the assets to download: %v", err)
	}

	installDir := filepath.Join(targetPath, "Unreal Engine - CSS")
	installerDir := filepath.Join(installDir, "Installer")

	err = os.MkdirAll(installerDir, 0666)
	if err != nil {
		return fmt.Errorf("could not create the directories for the path '%v': %v", installerDir, err)
	}

	for _, asset := range assetsToDownload {
		err := downloadAsset(ctx, client, asset, installerDir)
		if err != nil {
			return fmt.Errorf("could not download asset '%v': %v", asset.GetName(), err)
		}
	}

	err = runInstaller(installerDir, installDir, local)
	if err != nil {
		return fmt.Errorf("could not run the Unreal Engine installer: %v", err)
	}

	return nil
}

func getAssetsToDownload(ctx context.Context, client *github.Client) ([]*github.ReleaseAsset, error) {
	latestAssets, err := getLatestReleaseAssets(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("could not get the latest release's assets: %v", err)
	}

	assetsToDownload, err := filterAssets(latestAssets)
	if err != nil {
		return nil, fmt.Errorf("could not filter the assets to download: %v", err)
	}
	return assetsToDownload, nil
}

func getLatestReleaseAssets(ctx context.Context, client *github.Client) ([]*github.ReleaseAsset, error) {
	release, _, err := client.Repositories.GetLatestRelease(ctx, orgName, repoName)
	if err != nil {
		return nil, fmt.Errorf("could not get the latest release: %v", err)
	}

	assets, _, err := client.Repositories.ListReleaseAssets(ctx, orgName, repoName, release.GetID(), nil)
	if err != nil {
		return nil, fmt.Errorf("could not list the release's assets")
	}

	return assets, nil
}

func ensureGithubAccess(ctx context.Context, client *github.Client, githubToken string) error {
	_, _, err := client.Repositories.Get(ctx, orgName, repoName)
	if err != nil {
		return fmt.Errorf("could not get the repo: %v\nNOTE: This is most likely because you haven't joined the Epic Games organization. Please refer to the docs:\nhttps://docs.ficsit.app/satisfactory-modding/latest/Development/BeginnersGuide/dependencies.html#_unreal_engine_4_custom_engine", err)
	}

	return nil
}

func downloadAsset(ctx context.Context, client *github.Client, asset *github.ReleaseAsset, dir string) error {
	assetName := asset.GetName()

	fmt.Printf("Downloading asset %v\n", assetName)
	data, err := getAssetData(ctx, client, asset)
	if err != nil {
		return fmt.Errorf("could not get data for asset '%v': %v", assetName, err)
	}

	fmt.Printf("Writing asset '%v' to disk\n", assetName)
	err = writeAssetFile(dir, assetName, data)
	if err != nil {
		return fmt.Errorf("could not write asset '%v' to disk: %v", assetName, err)
	}
	return nil
}

func getAssetData(ctx context.Context, client *github.Client, asset *github.ReleaseAsset) ([]byte, error) {
	content, _, err := client.Repositories.DownloadReleaseAsset(ctx, orgName, repoName, asset.GetID(), http.DefaultClient)
	if err != nil {
		return nil, fmt.Errorf("could not start downloading asset '%v': %v", asset.GetName(), err)
	}
	defer content.Close()

	all, err := ioutil.ReadAll(content)
	if err != nil {
		return nil, fmt.Errorf("could not read the content of asset '%v': %v", asset.GetName(), err)
	}

	return all, nil
}

func writeAssetFile(targetDir, assetName string, data []byte) error {
	filename := filepath.Join(targetDir, assetName)
	return os.WriteFile(filename, data, 0666)
}

func runInstaller(installerDir, installDir string, local bool) error {
	filename := filepath.Join(installerDir, installerName)

	cmd := exec.Command(filename,
		"/SILENT",
		"/NORESTART",
	)
	if local {
		cmd.Args = append(cmd.Args, fmt.Sprintf(`/DIR=%v`, installDir))
	}

	return cmd.Run()
}

func makeGithubClient(ctx context.Context, accessToken string) *github.Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: accessToken})
	tc := oauth2.NewClient(ctx, ts)
	return github.NewClient(tc)
}

func filterAssets(assets []*github.ReleaseAsset) ([]*github.ReleaseAsset, error) {
	r := make([]*github.ReleaseAsset, 0)
	for _, asset := range assets {
		wanted, err := isAssetWanted(asset)
		if err != nil {
			return nil, fmt.Errorf("could not check if an asset is wanted:%v", err)
		}

		if wanted {
			r = append(r, asset)
		}
	}
	return r, nil
}

func isAssetWanted(asset *github.ReleaseAsset) (bool, error) {
	regex := `\.(exe|bin)`
	matched, err := regexp.MatchString(regex, asset.GetName())
	if err != nil {
		return false, fmt.Errorf("could not use regex on the asset name: %v", err)
	}
	return matched, nil
}
