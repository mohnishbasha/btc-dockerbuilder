package main

import (
    "fmt"
    "net/http"
    "os"
    "bytes"
    "strings"
    "time"
    
    "io/ioutil"
    "github.com/dimiro1/banner"
    
    "flag" 
    "gopkg.in/src-d/go-git.v4"
    . "gopkg.in/src-d/go-git.v4/_examples"
    "gopkg.in/src-d/go-git.v4/plumbing"
    
    "github.com/docker/docker/pkg/archive"
    "github.com/docker/docker/pkg/fileutils"
    "path"
    "path/filepath"
    
    "io"
    "github.com/docker/engine-api/client"
    "github.com/docker/engine-api/types"
    "golang.org/x/net/context"
    "bufio"

     "math/rand"
     "log"
     "github.com/DataDog/datadog-go/statsd"
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

    fmt.Println("Lightning fast docker build for ", url_param)
    
    // v2/github.com/mohnishbasha/hello-world/manifests/latest
    if strings.Contains(url_param, "v2") {
       url_param_1 := strings.ReplaceAll(url_param, "v2/", "")
       url_param_2 := strings.ReplaceAll(url_param_1, "/manifests/latest", "")
       Info(url_param_1)
       Info(url_param_2)
       url_param = url_param_2
    }
     

    statsd, err := statsd.New("127.0.0.1:8125")
    if err != nil {
       log.Fatal(err)
    }

    fmt.Fprintf(w, "%s", clone_n_dockerbuild(url_param, statsd))
    
    // fmt.Fprintf(w, " %s", "bob.run/" + url_param + ":latest")
    // ----------------------------------------
     
    // we need to buffer the body if we want to read it here and send it
    // in the request. 
    // body, err := ioutil.ReadAll(r.Body)
    // if err != nil {
    //    http.Error(w, err.Error(), http.StatusInternalServerError)
    //    return
    // }

    // redirect request to docker daemon
    
    // create a new url from the raw RequestURI sent by the client
    // new_url := fmt.Sprintf("%s://%s%s", "http", "bob.run:2375/", url_param)
    // proxyReq, err := http.NewRequest(r.Method, new_url, bytes.NewReader(body))

    // We may want to filter some headers, otherwise we could just use a shallow copy
    // proxyReq.Header = r.Header
    // new_client := &http.Client{}
    // new_resp, err := new_client.Do(proxyReq)
    // if err != nil {
    //     http.Error(w, err.Error(), http.StatusBadGateway)
    //     return
    // }
    // defer new_resp.Body.Close()
 
}


func clone_n_dockerbuild(git_url string, statsd *statsd.Client) (types.ImageBuildResponse) {
       
        domain_name := "bob.run"
        git_parts := strings.SplitN(git_url, "/", -1)
        fmt.Printf("\nSlice 1: %s", git_parts) 
        dockerImageTag := domain_name + "/" + git_url
        fmt.Printf("DockerImageTag: %s", dockerImageTag)
  
        cloneDirStr := fmt.Sprintf("%s%d%s", "clone-dir/",rand.Int(),git_url);
        cloneUrlStr := fmt.Sprintf("%s%d%s", "clone-url/",rand.Int(),git_url);
        shaStr := fmt.Sprintf("%s%d%s", "sha/",rand.Int(),git_url);

        cloneDirPtr := flag.String(cloneDirStr, "clone-dir/" + git_url, "Directory to clone")
        cloneUrlPtr := flag.String(cloneUrlStr, "https://" + git_url, "URL to clone")
	shaPtr := flag.String(shaStr, "", "sha to clone")
	flag.Parse()

	cloneOptions := git.CloneOptions{
		URL:           *cloneUrlPtr,
		ReferenceName: plumbing.ReferenceName("refs/heads/master"),
		SingleBranch:  true,
		Progress:      os.Stdout,
		Tags:          git.NoTags,
	}

	repo, err := git.PlainClone(*cloneDirPtr, false, &cloneOptions)
        if err != nil {
	   os.RemoveAll("clone-dir")
	}
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
    
        srcPath, err1 := filepath.Abs("clone-dir/" + git_url)
        if err1 != nil {
            fmt.Errorf("error1: '%s'", err1.Error())
        }

        // dockerfilePath, err2 := filepath.Abs("clone-dir/" + git_url + "/Dockerfile")
        // if err2 != nil {
        //    fmt.Errorf("error2: '%s'", err2.Error())
        // }  
        dockerfilePath := "Dockerfile" 
        fmt.Printf("srcPath=%q, dockerfilePath=%q\n", srcPath, dockerfilePath)
        

        Info("Tar file paths:  %s %s", srcPath, dockerfilePath)

        defer timeTrack(time.Now(), "dockerbuild-time", statsd, dockerImageTag)
        
        tarReader, err := CreateTarStream(srcPath, dockerfilePath)
	if err != nil {
            fmt.Errorf("error creating docker tar stream: '%s'", err.Error())
	}

        Info("Created tar stream ....")

        // initialize docker client & background context
        c := ensureDockerClient()
        netCtx := context.Background()

        Info("dockerfilepath: '%s'", dockerfilePath)
        Info("dockerImageTag: '%s'", dockerImageTag)	
        // set build options for docker build

        opts := types.ImageBuildOptions{
                Tags: []string{dockerImageTag + ":latest"},
		Dockerfile: dockerfilePath,
	}

        // invoke docker build
	buildResp, err := c.ImageBuild(netCtx,
		tarReader, opts)

	if err != nil {
		fmt.Errorf("error creating docker build image: '%s'", err.Error())
	}

	fmt.Printf("OSType=%q\n", buildResp.OSType)

        // docker image details
        bodyReader := bufio.NewReader(buildResp.Body)

	for {
		line, _, err := bodyReader.ReadLine()
		fmt.Printf("build: %q\n", string(line))
		if err == io.EOF {
			break
		} else if err != nil {
			fmt.Errorf("error read docker build image: '%s'", err.Error())
		}
	}
	fmt.Println("Image available: ", dockerImageTag)
         
        os.RemoveAll(srcPath)

        return buildResp
}


// generate a tar stream for ImageBuild API
// https://docs.docker.com/engine/api/v1.40/#operation/ImageBuild

func CreateTarStream(srcPath, dockerfilePath string) (io.ReadCloser, error) {

	excludes, err := parseDockerIgnore(srcPath)
	if err != nil {
		return nil, err
	}
       
        //excludes := []string{"."}
	includes := []string{"."}

	// If .dockerignore mentions .dockerignore or the Dockerfile
	// then make sure we send both files over to the daemon
	// because Dockerfile is, obviously, needed no matter what, and
	// .dockerignore is needed to know if either one needs to be
	// removed.  The deamon will remove them for us, if needed, after it
	// parses the Dockerfile.
	//
	// https://github.com/docker/docker/issues/8330
	//
	forceIncludeFiles := []string{".dockerignore", dockerfilePath}

	for _, includeFile := range forceIncludeFiles {
		if includeFile == "" {
			continue
		}
		keepThem, err := fileutils.Matches(includeFile, excludes)
		if err != nil {
			return nil, fmt.Errorf("cannot match .dockerfile: '%s', error: %s", includeFile, err)
		}
		if keepThem {
                        Info(includeFile)
			includes = append(includes, includeFile)
		}
	}

	if err := validateDockerContextDirectory(srcPath, excludes); err != nil {
		return nil, err
	}
	tarOpts := &archive.TarOptions{
		ExcludePatterns: excludes,
		IncludeFiles:    includes,
		Compression:     archive.Uncompressed,
		NoLchown:        true,
	}
	return archive.TarWithOptions(srcPath, tarOpts)
}

// validateContextDirectory checks if all the contents of the directory
// can be read and returns an error if some files can't be read.
// Symlinks which point to non-existing files don't trigger an error

func validateDockerContextDirectory(srcPath string, excludes []string) error {

	return filepath.Walk(filepath.Join(srcPath, "."), func(filePath string, f os.FileInfo, err error) error {
		// skip this directory/file if it's not in the path, it won't get added to the context
		if relFilePath, err := filepath.Rel(srcPath, filePath); err != nil {
			return err
		} else if skip, err := fileutils.Matches(relFilePath, excludes); err != nil {
			return err
		} else if skip {
			if f.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if err != nil {
			if os.IsPermission(err) {
				return fmt.Errorf("can't stat '%s'", filePath)
			}
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}

		// skip checking if symlinks point to non-existing files, such symlinks can be useful
		// also skip named pipes, because they hanging on open
		if f.Mode()&(os.ModeSymlink|os.ModeNamedPipe) != 0 {
			return nil
		}

		if !f.IsDir() {
			currentFile, err := os.Open(filePath)
			if err != nil && os.IsPermission(err) {
				return fmt.Errorf("no permission to read from '%s'", filePath)
			}
			currentFile.Close()
		}
		return nil
	})
}

func parseDockerIgnore(root string) ([]string, error) {
	var excludes []string
	ignore, err := ioutil.ReadFile(path.Join(root, ".dockerignore"))
	if err != nil && !os.IsNotExist(err) {
		return excludes, fmt.Errorf("error reading .dockerignore: '%s'", err)
	}
	excludes = strings.Split(string(ignore), "\n")

	return excludes, nil
}

func ensureDockerClient() *client.Client {
	c, err := client.NewEnvClient()
	if err != nil {
		fmt.Errorf("DOCKER_HOST not set?: %v", err)
	}
	return c
}

func timeTrack(start time.Time, name string, statsd *statsd.Client, dockerImageTag string) {
    elapsed := time.Since(start)
    Info("%s took %s", name, elapsed)
    
    //statsd, err := statsd.New("127.0.0.1:8125")
    //if err != nil {
    //   log.Fatal(err)
    //}
        
   statsd.Gauge("btc-dockerbuild." + name, float64(elapsed.Milliseconds()), []string{"Owner:tools","role:dockerbuildserver-hack","environment:dev","imageTag:"+dockerImageTag,"buildkit:false"}, 1)
   time.Sleep(1 * time.Second)
    
}
