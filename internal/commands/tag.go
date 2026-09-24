package commands

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"loki/internal/config"
	"loki/internal/core"
	"loki/internal/models"
	"loki/internal/storage"
)

func Tag(args []string) {
	lokiDir := findLokiDir()
	tagsDir := filepath.Join(lokiDir, "refs", "tags")

	if len(args) == 0 {
		listTags(tagsDir)
		return
	}

	var (
		listFlag     bool
		deleteFlag   bool
		annotateFlag bool
		forceFlag    bool
		message      string
		messageSet   bool
		posArgs      []string
	)

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-l" || arg == "--list" {
			listFlag = true
		} else if arg == "-d" || arg == "--delete" {
			deleteFlag = true
		} else if arg == "-a" || arg == "--annotate" {
			annotateFlag = true
		} else if arg == "-f" || arg == "--force" {
			forceFlag = true
		} else if arg == "-m" || arg == "--message" {
			if i+1 < len(args) {
				i++
				message = args[i]
				messageSet = true
			} else {
				fmt.Println("error: -m requires an argument")
				return
			}
		} else if strings.HasPrefix(arg, "-m=") {
			message = strings.TrimPrefix(arg, "-m=")
			messageSet = true
		} else if strings.HasPrefix(arg, "--message=") {
			message = strings.TrimPrefix(arg, "--message=")
			messageSet = true
		} else if strings.HasPrefix(arg, "-") {
			fmt.Printf("error: unknown flag %s\n", arg)
			return
		} else {
			posArgs = append(posArgs, arg)
		}
	}

	if listFlag && len(posArgs) == 0 {
		listTags(tagsDir)
		return
	}

	if deleteFlag {
		if len(posArgs) == 0 {
			fmt.Println("error: tag name required")
			return
		}
		for _, tagName := range posArgs {
			tagPath := filepath.Join(tagsDir, tagName)
			if _, err := os.Stat(tagPath); os.IsNotExist(err) {
				fmt.Printf("Error deleting tag %s\n", tagName)
				continue
			}
			err := os.Remove(tagPath)
			if err != nil {
				fmt.Printf("Error deleting tag %s\n", tagName)
			} else {
				fmt.Printf("Deleted tag %s\n", tagName)
			}
		}
		return
	}

	if len(posArgs) == 0 {
		listTags(tagsDir)
		return
	}

	tagName := posArgs[0]
	target := "HEAD"
	if len(posArgs) > 1 {
		target = posArgs[1]
	}

	commitHash, err := resolveCommit(lokiDir, target)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	if commitHash == "" {
		fmt.Println("No commits yet")
		return
	}

	tagPath := filepath.Join(tagsDir, tagName)
	if _, err := os.Stat(tagPath); err == nil && !forceFlag {
		fmt.Printf("fatal: tag '%s' already exists\n", tagName)
		return
	}

	if err := os.MkdirAll(tagsDir, 0755); err != nil {
		fmt.Println("Error creating tags directory")
		return
	}

	if annotateFlag || messageSet {
		cwd, _ := os.Getwd()
		repoRoot, _ := core.IsRepoInitialized(cwd)
		tagger, email := getTaggerInfo(repoRoot)

		if !messageSet {
			message = fmt.Sprintf("Tag %s", tagName)
		}

		tagModel := &models.Tag{
			Object:  commitHash,
			Type:    models.CommitObject,
			Name:    tagName,
			Tagger:  tagger,
			Email:   email,
			Date:    time.Now().Unix(),
			Message: message,
		}

		store := storage.NewFileStorage(lokiDir)
		tagObjHash := store.WriteObject(tagModel.Serialize())

		err := os.WriteFile(tagPath, []byte(tagObjHash+"\n"), 0644)
		if err != nil {
			fmt.Println("Error creating tag")
			return
		}
		return
	}

	// Lightweight Tag
	err = os.WriteFile(tagPath, []byte(commitHash+"\n"), 0644)
	if err != nil {
		fmt.Println("Error creating tag")
		return
	}
}

func listTags(tagsDir string) {
	entries, err := os.ReadDir(tagsDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			fmt.Println(entry.Name())
		}
	}
}

func findLokiDir() string {
	cwd, err := os.Getwd()
	if err == nil {
		if repoRoot, ok := core.IsRepoInitialized(cwd); ok {
			return filepath.Join(repoRoot, ".loki")
		}
	}
	return ".loki"
}

func getTaggerInfo(repoRoot string) (string, string) {
	cfg := config.NewConfig()
	_ = cfg.Load(repoRoot)

	tagger := cfg.Get("user.name")
	email := cfg.Get("user.email")

	if tagger == "" {
		tagger = "loki"
	}
	if email == "" {
		email = "loki@local"
	}
	return tagger, email
}

func resolveCommit(lokiDir string, target string) (string, error) {
	if target == "" || target == "HEAD" {
		return resolveHeadCommit(lokiDir)
	}

	// 1. Check branch
	branchPath := filepath.Join(lokiDir, "refs", "heads", target)
	if hashData, err := os.ReadFile(branchPath); err == nil {
		hash := strings.TrimSpace(string(hashData))
		if hash != "" {
			return hash, nil
		}
	}

	// 2. Check tag (dereference annotated tag if needed)
	tagPath := filepath.Join(lokiDir, "refs", "tags", target)
	if hashData, err := os.ReadFile(tagPath); err == nil {
		hash := strings.TrimSpace(string(hashData))
		if hash != "" {
			store := storage.NewFileStorage(lokiDir)
			if objData, err := store.ReadObject(hash); err == nil && bytes.HasPrefix(objData, []byte("tag ")) {
				if tagObj, err := models.ParseTag(objData); err == nil && tagObj.Object != "" {
					return tagObj.Object, nil
				}
			}
			return hash, nil
		}
	}

	// 3. Check if target is an object hash
	if len(target) >= 4 {
		store := storage.NewFileStorage(lokiDir)
		if _, err := store.ReadObject(target); err == nil {
			return target, nil
		}
	}

	return "", fmt.Errorf("fatal: Not a valid object name %s", target)
}

func resolveHeadCommit(lokiDir string) (string, error) {
	headPath := filepath.Join(lokiDir, "HEAD")
	headData, err := os.ReadFile(headPath)
	if err != nil {
		return "", err
	}
	ref := strings.TrimSpace(string(headData))
	if strings.HasPrefix(ref, "ref:") {
		refTarget := strings.TrimSpace(strings.TrimPrefix(ref, "ref:"))
		refPath := filepath.Join(lokiDir, refTarget)
		refHashData, err := os.ReadFile(refPath)
		if err != nil {
			if os.IsNotExist(err) {
				return "", nil
			}
			return "", err
		}
		return strings.TrimSpace(string(refHashData)), nil
	}
	return ref, nil
}
