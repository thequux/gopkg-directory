Usage
=====

Create a config file with one line for each repository. Each line should
consist of three tab-separated fields: import path prefix, vcs type, and
repository path. For example, this tool serves its own import path with:

    gopkg.thequux.com/tools/gopkg-directory	git	https://github.com/thequux/gopkg-directory

Name this file "pkg-directory.conf". (or don't; there's a flag for that). Then,
start the server with

    gopkg-directory --config=pkg-directory.conf --fcgi=localhost:6061

(change the parameters to suit, of course). You will also need a webserver in
front of Go; I use lighttpd. It should be easy enough to configure, but here's
a head start:

    HTTP["host"] == "gopkg.thequux.com" {
        fastcgi.server = (
          "/" => ((
                "host" => "127.0.0.1",
                "port" => "6061",
                "check-local" => "disable"
                ))
        )
    }

I hope you find this useful!
