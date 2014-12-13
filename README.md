<pre>
        _,
       | | o             |
  __,  | | ,   _  _    __|
./  |  |/  |  / |/ |  /  | .
 \_/|_/|__/|_/  |  |_/\_/|_/
       |\
       |/
</pre>

`afind` | distributed code search
---------------------------------
[![Build Status](https://travis-ci.org/andaru/afind.svg?branch=develop)](https://travis-ci.org/andaru/afind)
 
afind is a distributed code search service based on Russ Cox's
codesearch libraries.  A daemon (running either as a backend
or a master/backend) and a command line interface are provided. The
network daemon 'afindd' offers a REST interface and an RPC
interface used by other afindd instances.

Installation
------------

    $ go get -u github.com/andaru/afind/...
    % go install github.com/andaru/afind/...

Running the afindd daemon
-------------------------
To start, first decide whether you'd like the repository indices
stored inside the roots of the repositories (the default):

    $ afindd

Or if you'd prefer all indices are written in the same folder, in this case, /tmp/afind/indices/:

    $ afindd -index_in_repo=false -index_root=/tmp/afind/indices

You can also provide a file name for persistent storage of the backing store. You'll use this when
you're running afindd and want it to reload existing configuration at startup.

    $ afindd -dbfile="/tmp/afind/backing_store.json"

Now afind is running, you can index some text or source code, and make queries.

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
If the `-httpbind` argument is supplied, afindd will operate a JSON/REST
interface for access to repository metadata, search and indexing.
Here's some quick notes about the URL and request formats.

### Indexing

If using `-httpbind=:30880` for afindd, the `afind` CLI command:

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

    $ curl -d '{"re": "foobar"}' http://localhost:30880/search
    
To search in source repos for a particular project:

    $ curl -d '{"re": "foobar", meta: {"project": "mainline"}}' http://localhost:30880/search


Contact
-------
Please open issues on GitHub if you would like new features or wish to report bugs.
