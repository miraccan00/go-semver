package main

import (
	"fmt"
	"os"

	"new-semver/internal/semver"
)

func main() {
	if !semver.CheckGitRepoExists() {
		fmt.Fprintln(os.Stderr, ".git klasörü bulunamadı, bir git reposunda çalıştırmalısınız.")
		os.Exit(1)
	}

	v, err := semver.ReadVersion()
	if err != nil {
		nextVer, ymlErr := semver.ReadNextVersionFromYML()
		if ymlErr != nil {
			fmt.Println("Error reading version and GitVersion.yml:", err, ymlErr)
			os.Exit(1)
		}
		v, err = semver.ParseVersion(nextVer)
		if err != nil {
			fmt.Println("Invalid next-version in GitVersion.yml")
			os.Exit(1)
		}
	}

	msg, err := semver.GetLastCommitMessage()
	if err != nil {
		// Commit yoksa, GitVersion.yml'den semantic versiyon üret
		if _, statErr := os.Stat("GitVersion.yml"); os.IsNotExist(statErr) {
			fmt.Fprintln(os.Stderr, "Hiç commit bulunamadı ve GitVersion.yml yok. Lütfen bir GitVersion.yml oluşturun veya ilk commitinizi yapın.")
			os.Exit(1)
		}
		nextVer, ymlErr := semver.ReadNextVersionFromYML()
		if ymlErr != nil {
			fmt.Fprintln(os.Stderr, "GitVersion.yml okunamadı:", ymlErr)
			os.Exit(1)
		}
		v, parseErr := semver.ParseVersion(nextVer)
		if parseErr != nil {
			fmt.Fprintln(os.Stderr, "GitVersion.yml içindeki next-version hatalı:", parseErr)
			os.Exit(1)
		}
		info := semver.BuildVersionInfo(v)
		semver.PrintJSON(info)
		return
	}

	before := v
	semver.BumpByCommitMessage(&v, msg)

	if v != before {
		err = semver.WriteVersion(v)
		if err != nil {
			fmt.Println("Error writing version:", err)
			os.Exit(1)
		}
	}

	info := semver.BuildVersionInfo(v)
	semver.PrintJSON(info)
}
