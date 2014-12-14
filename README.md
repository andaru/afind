<pre>
        _,
       | | o             |
  __,  | | ,   _  _    __|
./  |  |/  |  / |/ |  /  | .
 \_/|_/|__/|_/  |  |_/\_/|_/
       |\
       |/
</pre>
a codesearch service
--------------------
[![Build Status](https://travis-ci.org/andaru/afind.svg?branch=develop)](https://travis-ci.org/andaru/afind)
 
afind is a distributed code search service based on Russ Cox's
codesearch libraries.  A daemon (running either as a backend
or a master/backend) and a command line interface are provided. The
network daemon `afindd` offers a REST interface and an RPC
interface used by other afindd instances.

Installation
------------

    $ go get -u github.com/andaru/afind/...
    % go install github.com/andaru/afind/...

Running the `afindd` daemon
---------------------------
First, decide whether you'd like the repository index files
stored in the *root directory of the repository* (the default):

    $ afindd

If you'd prefer all indices are *written in the same folder*, e.g., `/tmp/afind/indices/`:

    $ afindd -index_in_repo=false -index_root=/tmp/afind/indices

`afindd` can read and write its repositories to disk if you provide the `-dbfile` argument.
You'll commonly use this flag in production:

    $ afindd -dbfile="/tmp/afind/backing_store.json"

Now that afind is running, you can index some source code and make queries of the indices.

Distributed operation
---------------------
The afind service has a simple query strategy which leads to a simple configuration of processes in a distributed system. Indexing or search queries directed for a single
host will be proxies by an `afindd` job once only. This is similar to recursive queries
in the domain name system. Index requests name their intended target by setting the `host`
key in the `IndexRequest` metadata. The common client search request case 
(where no repositories are selected) uses all repositories available
to the `afindd` servicing the request. For this reason, it is suggested that general client
requests be sent to a "front end" `afindd` instance, having a complete 
view of available repositories for searches. Index requests that are proxied to a remote
which does have the repository already available will be back-filled on the front-end
originally servicing the request.

Simply said:

 1. Start a back-end (e.g., `afindd`) on hosts containing source data to index.
 2. Start a front-end with HTTP server (e.g., `afindd -http=:80`) on a well known host, e.g., `afind.$DOMAIN`
 3. Send client HTTP/RPC requests to `afind.$DOMAIN`

Indexing repositories
---------------------

The `afind` command line tool is used to index, view and search reopsitories:

    $ afind index -D project=foobar ID /path/to/root subdir1 subdir2/subdir3

The above command will create and index a new repository with Key
`ID`, whose root path is `root` and that will index recursively the
subdirs provided under root. Subdirs *must be non-absolute* (i.e.,
have no leading `/`). To specify everything in the root, use the
single subdirectory `.`.  The indexed repository will also have the
additional metadata key `project` with value `foobar`, in addition
to the default `host` and `port.*` metadata values inserted by the
server to indicate where the repository is located.

Searching
---------
Once you've indexed some code, search for it across all repos known to
the afind service like so:

    $ afind search foo.*bar

HTTP server
-----------
If the `-http` argument is supplied, afindd will operate a JSON/REST
interface for access to repository metadata, search and indexing.
Here's some quick notes about the URL and request formats.

### Indexing

If we ran the server as `afindd -http=:30880` for afindd, the `afind` CLI command:

    $ afind index -D project=mainline 123 /var/proj/root src/dir1 src/dir2

is equivalent to the following HTTP request:

    $ curl -d '{"key": "123", \
                "root": "/var/proj/root", \
                "meta": {"project": "mainline"}, \
                "dirs": ["src/dir1", "src/dir2"]}' http://localhost:30880/repo

### Searching

The `afind` CLI command:

    $ afind search foobar

Is equivalent to the HTTP request:

    $ curl -d '{"q": "foobar"}' http://localhost:30880/search
    
To search in source repos for a particular project, like `mainline`:

    $ curl -d '{"q": "foobar", meta: {"project": "mainline"}}' http://localhost:30880/search


Contact
-------
Please open issues on GitHub if you would like new features or wish to report bugs.
