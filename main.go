// The config file is very simple; it is a sequence of lines, each
// with three fields with a tab between them. The first field is an
// import path prefix, the second is the VCS in use (eg, git, hg), and
// the third is a repository URL. Leading and trailing whitespace is
// ignored. Everything on a line after a hash ("#'") is ignored, as
// are blank lines.
// 
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/fcgi"
	"net/http/httputil"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

var config = flag.String("config", "pkg-directory.conf", "Path to a pkg-directory config file")
var fcgi_addr = flag.String("fcgi", "localhost:6061", "The port to listen for fastcgi requests on")

type DirMap struct {
	Repository string
	Path       string
	VCS        string
	Parent     *DirMap
	subDirs    map[string]*DirMap
}

var dirMap = &DirMap{
	Repository: "",
	VCS:        "",
	Path:       "",
	Parent:     nil,
	subDirs:    map[string]*DirMap{},
}

func init() {

}

func (d *DirMap) GetSubdir(path string, uniqify bool) *DirMap {
	cur := d
	// This doesn't stop people from doing stupid shit like
	// "foo/../bar", but not a security vuln as no filesystem
	// accesses happen.
	for _, component := range strings.FieldsFunc(path, func(c rune) bool { return c == '/' }) {
		if res, ok := cur.subDirs[component]; ok {
			cur = res
		} else if uniqify {
			cur.subDirs[component] = &DirMap{
				Repository: "",
				Path:       cur.Path + "/" + component,
				Parent:     cur,
				subDirs:    map[string]*DirMap{},
			}
			cur = cur.subDirs[component]
		} else {
			return cur
		}
	}
	return cur
}

func LoadConfig(cfgpath string) error {
	// TODO: make the config file more complete; eg, where to cache the checked out repo, etc
	log.Print("loading config")
	rawfile, err := os.Open(cfgpath)
	if err != nil {
		return err
	}
	defer rawfile.Close()
	file := bufio.NewReader(rawfile)

	newDM := &DirMap{
		Repository: "",
		VCS:        "",
		Path:       "",
		Parent:     nil,
		subDirs:    map[string]*DirMap{},
	}

	lno := 0
	for {
		line, err := file.ReadString('\n')
		lno++
		if err != nil && err != io.EOF {
			return err
		} else {
			line := strings.TrimSpace(strings.Split(line, "#")[0])
			if line == "" {
				if err != nil {
					break
				}
				continue
			}
			fields := strings.Fields(line)
			if len(fields) != 3 {
				log.Printf("Invalid config: wrong number of fields on line %d", lno)
				return nil
			}
			prefix := fields[0]
			vcs := fields[1]
			repo := fields[2]

			dm := newDM.GetSubdir(prefix, true)
			dm.Repository = repo
			dm.Path = prefix
			dm.VCS = vcs

			if err != nil {
				break
			}
		}
	}
	dirMap = newDM
	return nil
}
func ServeDirMap(w http.ResponseWriter, req *http.Request) {
	dirMap.ServeHTTP(w, req)
}

func (d *DirMap) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	host := req.URL.Host
	if host == "" {
		host = req.Header.Get("Host")
	}
	if host == "" {
		host = req.Header.Get("X-Host")
	}
	path := host + "/" + req.URL.Path
	mapitem := dirMap.GetSubdir(path, false)
	for {
		if mapitem == nil {
			w.WriteHeader(404)
			w.Header().Add("content-type", "text/plain")
			dr, _ := httputil.DumpRequest(req, true)
			w.Write(dr)
			return
		} else if mapitem.Repository == "" {
			mapitem = mapitem.Parent
		} else {
			break
		}
	}
	w.Header().Add("content-type", "text/html")

	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
 <head>
  <meta name="go-import" content="%s %s %s">
 </head>
</html>`, mapitem.Path, mapitem.VCS, mapitem.Repository)
}


func main() {
	flag.Parse()
	err := LoadConfig(*config)
	if dirMap == nil {
		log.Fatal("No valid config; exiting")
	}
	if err != nil {
		panic(err)
	}

	syscallChan := make(chan os.Signal, 1)
	signal.Notify(syscallChan, syscall.SIGUSR1)

	go func() {
		for _ = range syscallChan {
			LoadConfig(*config)
		}
	}()
	log.Print("Starting up")


	http.HandleFunc("/", ServeDirMap)

	
	// fastcgi
	addr, err := net.ResolveTCPAddr("tcp", *fcgi_addr)
	if err != nil {
		log.Fatal(err)
	}
	sock, err := net.ListenTCP("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	panic(fcgi.Serve(sock, nil))
	/*

		panic(http.ListenAndServe(":6061", dirMap))
	*/
}
