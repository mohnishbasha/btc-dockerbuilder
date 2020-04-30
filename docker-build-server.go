package main

import (
    "fmt"
    "net/http"
    "os"
    "bytes"
    "strings"
    
    "io/ioutil"
    "github.com/dimiro1/banner"
    
    "flag" 
    "gopkg.in/src-d/go-git.v4"
    . "gopkg.in/src-d/go-git.v4/_examples"
    "gopkg.in/src-d/go-git.v4/plumbing"
)


type GithubUrl struct {
	GithubDomain string
	GithubRepo  string
}


func main() {
    isEnabled := true
    isColorEnabled := true
    fmt.Println("Started Confluent Docker Build Server ...")
    
    // print bob ascii 
    bob_bytes, err := ioutil.ReadFile("resources/bob.txt") // just pass the file name
    if err != nil {
        fmt.Print(err)
    }
    bob_str := string(bob_bytes)
    banner.Init(os.Stdout, isEnabled, isColorEnabled, bytes.NewBufferString(bob_str))
    
    fmt.Println("")
    fmt.Println("Confluent Docker Build Server started ... accepting requests")
    
    // print bob the cat 
    cat_bytes, err := ioutil.ReadFile("resources/cat.txt") // just pass the file name
    if err != nil {
        fmt.Print(err)
    }
    cat_str := string(cat_bytes)
    banner.Init(os.Stdout, isEnabled, isColorEnabled, bytes.NewBufferString(cat_str))

    http.HandleFunc("/", DockerConfluentBuildServer)
    http.ListenAndServe(":8080", nil)
}

func DockerConfluentBuildServer(w http.ResponseWriter, r *http.Request) {
    
    url_param := r.URL.Path[1:]

    if !strings.Contains(url_param, "github.com") {
        fmt.Println("Invalid Github Repo!")
        return
    }

    fmt.Fprintf(w, "Lightning fast building for  %s", url_param)

    git_clone(url_param)
}


func git_clone(git_url string) {
        
        git_parts := strings.SplitN(git_url, "/", -1)
        fmt.Printf("\nSlice 1: %s", git_parts) 
 

        cloneDirPtr := flag.String("clone-dir", "clone-dir/" + git_url, "Directory to clone")
	cloneUrlPtr := flag.String("clone-url", "https://" + git_url, "URL to clone")
	shaPtr := flag.String("sha", "", "sha to clone")
	flag.Parse()

	cloneOptions := git.CloneOptions{
		URL:           *cloneUrlPtr,
		ReferenceName: plumbing.ReferenceName("refs/heads/master"),
		SingleBranch:  true,
		Progress:      os.Stdout,
		Tags:          git.NoTags,
	}
	repo, err := git.PlainClone(*cloneDirPtr, false, &cloneOptions)
	CheckIfError(err)
	reference, err := repo.Head()
	CheckIfError(err)
	Info("Cloned! Head at %s", reference)

	workTree, err := repo.Worktree()
	CheckIfError(err)

	err = workTree.Reset(&git.ResetOptions{
		Commit: plumbing.NewHash(*shaPtr),
		Mode:   git.HardReset,
	})
	CheckIfError(err)
	Info("Hard reseted to %s", *shaPtr)

	status, err := workTree.Status()
	CheckIfError(err)
	Info("Status after reset: %s", status)

	repo.Storer.Index()
}
