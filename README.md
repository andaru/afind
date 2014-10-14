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

afind is a distributed code search engine based on Russ Cox's
codesearch libraries.  A network daemon (running either as a backend
or a master/backend) and a command line interface are provided. The
network daemon 'afindd' offers a REST interface and a backend
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

Now afind is running, you can index some text and make queries

Indexing repositories
---------------------

The `afind` command line tool is used to index, view and search reopsitories:

    $ afind -D project=foobar index id /path/to/root subdir1 subdir2/subdir3

The above command will create and index a new repository with Key
`id`, whose root path is `root` and that will index recursively the
subdirs provided under root. Subdirs *must be non-absolute* (i.e.,
have no leading `/`). To specify everything in the root, use the
single subdirectory `.`.  The indexed repository will also have the
additional metadata key `project` with value `foobar`, in addition
to the default `host` and `port.*` metadata values inserted by the
server to indicate where the repository is located.

Searcing
--------

Once you've indexed some code, to search using the `afind` tool:

    $ afind search foobar

Contact
-------
Please open issues on GitHub if you would like new features or wish to report bugs.
