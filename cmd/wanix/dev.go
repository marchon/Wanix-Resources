package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/toolkit-go/engine/fs"
	"tractor.dev/toolkit-go/engine/fs/watchfs"
	"tractor.dev/wanix/boot"
	"tractor.dev/wanix/internal/httpfs"
	"tractor.dev/wanix/kernel/web/gwutil"
)

func devCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "dev",
		Short: "start wanix dev server",
		Run: func(ctx *cli.Context, args []string) {
			// are we in a wanix checkout?
			found, err := fs.Exists(os.DirFS("."), "cmd/wanix/main.go")
			if err != nil {
				fatal(err)
			}
			if !found {
				fatal(fmt.Errorf("not in a wanix working directory"))
			}

			runServer()
		},
	}
	return cmd
}

func loggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		// Call the next handler in the chain
		next.ServeHTTP(w, r)
		// Log the request details
		log.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(start))
	})
}

func runServer() {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	basePath := ""
	log.Printf("Serving WANIX dev server at http://localhost:7777%s ...\n", basePath)

	mux := http.NewServeMux()
	mux.Handle("/auth/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/" {
			domain := os.Getenv("AUTH0_DOMAIN")
			clientID := os.Getenv("AUTH0_CLIENTID")
			if domain == "" || clientID == "" {
				log.Fatal("Auth was used with Auth0 env vars set")
			}
			d, err := os.ReadFile("./boot/site/auth/index.html")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			d = bytes.ReplaceAll(d, []byte("AUTH0_DOMAIN"), []byte(domain))
			d = bytes.ReplaceAll(d, []byte("AUTH0_CLIENTID"), []byte(clientID))
			if _, err := w.Write(d); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		http.StripPrefix("/auth/", http.FileServer(http.Dir(dir+"/boot/site/auth"))).ServeHTTP(w, r)
	}))

	mux.Handle(fmt.Sprintf("%s/sys/dev/", basePath), http.StripPrefix(fmt.Sprintf("%s/sys/dev/", basePath), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gwutil.FileTransformer(watchfs.New(os.DirFS(dir)), httpfs.FileServer).ServeHTTP(w, r)
	})))
	mux.Handle(fmt.Sprintf("%s/wanix-kernel.gz", basePath), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(boot.Dir, "kernel.gz")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		reader := bytes.NewReader(data)
		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("Content-Disposition", `attachment; filename="wanix-kernel.gz"`)
		http.ServeContent(w, r, "wanix-kernel.gz", time.Now(), reader)
	}))
	mux.Handle(fmt.Sprintf("%s/wanix-initfs.gz", basePath), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(boot.Dir, "initfs.gz")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		reader := bytes.NewReader(data)
		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("Content-Disposition", `attachment; filename="wanix-initfs.gz"`)
		http.ServeContent(w, r, "wanix-initfs.gz", time.Now(), reader)
	}))
	mux.Handle(fmt.Sprintf("%s/wanix-bootloader.js", basePath), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/javascript")
		bl, err := buildBootloader()
		fatal(err)
		w.Write(bl)
	}))
	mux.Handle(fmt.Sprintf("%s/", basePath), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.StripPrefix(fmt.Sprintf("%s/", basePath), http.FileServer(http.Dir(dir+"/boot/site"))).ServeHTTP(w, r)
	}))
	if err := http.ListenAndServe(":7777", loggerMiddleware(mux)); err != nil {
		log.Fatal(err)
	}
}
