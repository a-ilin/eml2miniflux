eml2miniflux
============

This tool injects news entries stored as EML files into Miniflux database.

It may be useful for the migration of existent news from Mozilla Thunderbird or Opera to Miniflux.


# EML import process

The EML import process may look as following:
1. Choose or create a feed in Miniflux which will be associated with the migrated news.
2. Export news from Thunderbird in EML format.
3. Import the EML files into Miniflux using this tool.


The step (3) may be done in different ways.
Refer to the command line specification below for tool parameters and execution examples.


## Feed matching

During the EML import a relation between the imported news entries and a news feed must be established. `eml2miniflux` supports following modes.


### One feed for all EML files

This mode is useful when all exported EML are referring to the same news source.
In `eml2miniflux` it is supported with `-feed` command line argument.


### Different feeds for different EML files

This mode is useful when exported EML are referring to different news sources.
It requires providing a text file, `feed map`, to the tool to make a correct match of a EML file to a feed.
In `eml2miniflux` it is supported with `-feedmap` command line argument.
Refer to the command line specification below for the feed map format.


# Compilation

Run the following commands to compile the tool:
```sh
git submodule update --init --recursive
make
```

The compiled binary would be available under `bin` subdirectory.


# Command line
```sh
eml2miniflux --help
Usage: eml2miniflux <options> <EML_file | directory | dump_json_file>
Import EML files into Miniflux.

Embedded Miniflux version: 2.0.43

Options:
  -batch int
        Pseudo-amount of messages to commit to the database at a time (default 1000)
  -dburl string
        (mandatory) Database connection URL, ex.: postgres://miniflux:secret@db/miniflux?sslmode=disable
  -dry
        Dry run: read EML and attempt necessary transformations, but do not commit changes to the database
  -dump string
        Write extracted EML entries dump to a specified file
  -feed string
        (mandatory?) URL of the feed to assign the entries; must be specified the feed URL or the feed map file
  -feedmap string
        (mandatory?) Feed map file; must be specified the feed URL or the feed map file
  -mark
        Mark the inserted entries as read
  -quiet
        Suppress output about unmatched messages
  -remove
        Remove existent entries with matched user and hash from the database
  -retries int
        Amount of attempts to run a database transaction (default 10)
  -update
        Update existent entries in the database
  -user string
        (mandatory) Name of the user of the entries

Example using the feed URL:
  eml2miniflux -dburl=postgres://miniflux:password@server:5432/miniflux?sslmode=disable -user=john -feed=https://example.com/rss.xml /path/to/rss.eml

Example using the feed map file:
  eml2miniflux -dburl=postgres://miniflux:password@server:5432/miniflux?sslmode=disable -user=john -feedmap=/path/to/feed_helper.txt /path/to/directory/with/emls

Example using the JSON dump, with 2 steps: parsing and importing:
  eml2miniflux -dburl=postgres://miniflux:password@server:5432/miniflux?sslmode=disable -user=john -feed=https://example.com/rss.xml -dump=entries.json -dry /path/to/directory/with/emls
  eml2miniflux -dburl=postgres://miniflux:password@server:5432/miniflux?sslmode=disable entries.json

FEED MAP
  Feed map file contains URL substitution rules for matching of multiple EML files within a directory into multiple feeds.
  Empty lines, or lines starting with # symbol are ignored.
  URL substitution is defined as following:
    substring-of-EML-URL => defined-feed-URL|none
  When 'none' value is used, the EML is ignored without producing warnings.

  Example of a feed map file:
    # EML with xkcd.com in URL should go to the corresponding feed
    xkcd.com => https://xkcd.com/rss.xml

    # EML with devblogs.technet.com in URL should go to the feed of VS
    devblogs.technet.com => https://devblogs.microsoft.com/visualstudio/feed/

    # EML with blogs.technet.com in URL should be ignored
    # Notice schema in the beginning required to avoid undesired match with entries having devblogs.technet.com in URL
    http://blogs.technet.com => none

TROUBLESHOOT
  Error 'Error on processing file: some.eml: feed not found for URL: http://some.url' specifies that the URL cannot be matched to a feed.
  Add the URL to a feed map file with the corresponding feed URL substitution, or use '-feed' option.

  Error 'Failed: you must run the SQL migrations' specifies the difference of the installed Miniflux version and the used one in this tool.
  In order to proceed either the installed Miniflux must be updated, or the submodule 'sub/miniflux' of this tool.

  Error 'Failed: cannot update entries in database: store: unable to start transaction: EOF' specifies that network connection to the database is unstable.
  Use parameter '-retries' to increase amount of attempts, or connect to a stable network.
```

# Known issues

## Miniflux database version mismatch

This tool relies on a specific version of Miniflux database. Before proceeding with a commit to the database it is recommended to ensure that the installed Miniflux version corresponds to the version of Miniflux being integrated within `eml2miniflux`.

In case of a version mismatch the recommended way to proceed is an upgrade of an installed database version, or an upgrade of the integrated Miniflux within this tool.

The integrated Miniflux version may be found as following:
```sh
eml2miniflux --help
Usage: eml2miniflux <options> <EML_file | directory | dump_json_file>
Import EML files into Miniflux.

Embedded Miniflux version: 2.0.43
...
```

## Notice on the integrated Miniflux

This tool relies on Miniflux source code. Miniflux is being added as a git submodule under `sub/miniflux`. The recommended Golang approach of integrating the Go modules by means of `go.mod` is not available here, due to incorrect module naming within https://github.com/miniflux/v2/blob/main/go.mod file: Golang requires that modules starting with version `v2` and further have name suffix `/v2`: https://go.dev/blog/v2-go-modules


# License

The source code of `eml2miniflux` is provided under Apache license.
