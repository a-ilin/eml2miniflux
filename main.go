package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/a-ilin/eml2miniflux/eml2miniflux"
	"github.com/a-ilin/eml2miniflux/util"
	"miniflux.app/database"
	"miniflux.app/model"
	"miniflux.app/storage"
)

type Config struct {
	DatabaseUrl string
	MessageFile string
	MessageType int
	Username    string
	Feed        string
	FeedMapFile string
	MarkRead    bool
	Update      bool
	Remove      bool
	BatchSize   int
	Retries     int
	DryRun      bool
	Quiet       bool
	DumpFile    string
}

const (
	MESSAGE_EML = iota
	MESSAGE_JSON
	MESSAGE_DIRECTORY
)

var (
	// set via LDFLAGS in Makefile
	MinifluxVersion string
)

// https://stackoverflow.com/a/25113485
func permutateArgs(args []string) int {
	args = args[1:]
	optind := 0

	for i := range args {
		if args[i][0] == '-' {
			tmp := args[i]
			args[i] = args[optind]
			args[optind] = tmp
			optind++
		}
	}

	return optind + 1
}

func printUsage() {
	prog := filepath.Base(os.Args[0])
	fmt.Fprintf(os.Stderr, "Usage: %s <options> <EML_file | directory | dump_json_file>\n", prog)
	fmt.Fprintf(os.Stderr, "Import EML files into Miniflux.\n")
	fmt.Fprintf(os.Stderr, "\nEmbedded Miniflux version: %s\n", MinifluxVersion)
	fmt.Fprintf(os.Stderr, "\nOptions:\n")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\nExample using the feed URL:\n")
	fmt.Fprintf(os.Stderr, "  %s -dburl=postgres://miniflux:password@server:5432/miniflux?sslmode=disable -user=john -feed=https://example.com/rss.xml /path/to/rss.eml\n", prog)
	fmt.Fprintf(os.Stderr, "\nExample using the feed map file:\n")
	fmt.Fprintf(os.Stderr, "  %s -dburl=postgres://miniflux:password@server:5432/miniflux?sslmode=disable -user=john -feedmap=/path/to/feed_helper.txt /path/to/directory/with/emls\n", prog)
	fmt.Fprintf(os.Stderr, "\nExample using the JSON dump, with 2 steps: parsing and importing:\n")
	fmt.Fprintf(os.Stderr, "  %s -dburl=postgres://miniflux:password@server:5432/miniflux?sslmode=disable -user=john -feed=https://example.com/rss.xml -dump=entries.json -dry /path/to/directory/with/emls\n", prog)
	fmt.Fprintf(os.Stderr, "  %s -dburl=postgres://miniflux:password@server:5432/miniflux?sslmode=disable entries.json\n", prog)
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "FEED MAP\n")
	fmt.Fprintf(os.Stderr, "  Feed map file contains URL substitution rules for matching of multiple EML files within a directory into multiple feeds.\n")
	fmt.Fprintf(os.Stderr, "  Empty lines, or lines starting with # symbol are ignored.\n")
	fmt.Fprintf(os.Stderr, "  URL substitution is defined as following:\n")
	fmt.Fprintf(os.Stderr, "    substring-of-EML-URL => defined-feed-URL|none\n")
	fmt.Fprintf(os.Stderr, "  When 'none' value is used, the EML is ignored without producing warnings.\n")
	fmt.Fprintf(os.Stderr, "\n  Example of a feed map file:\n")
	fmt.Fprintf(os.Stderr, "    # EML with xkcd.com in URL should go to the corresponding feed\n")
	fmt.Fprintf(os.Stderr, "    xkcd.com => https://xkcd.com/rss.xml\n")
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "    # EML with devblogs.technet.com in URL should go to the feed of VS\n")
	fmt.Fprintf(os.Stderr, "    devblogs.technet.com => https://devblogs.microsoft.com/visualstudio/feed/\n")
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "    # EML with blogs.technet.com in URL should be ignored\n")
	fmt.Fprintf(os.Stderr, "    # Notice schema in the beginning required to avoid undesired match with entries having devblogs.technet.com in URL\n")
	fmt.Fprintf(os.Stderr, "    http://blogs.technet.com => none\n")
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "TROUBLESHOOT\n")
	fmt.Fprintf(os.Stderr, "  Error 'Error on processing file: some.eml: feed not found for URL: http://some.url' specifies that the URL cannot be matched to a feed.\n")
	fmt.Fprintf(os.Stderr, "  Add the URL to a feed map file with the corresponding feed URL substitution, or use '-feed' option.\n")
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "  Error 'Failed: you must run the SQL migrations' specifies the difference of the installed Miniflux version and the used one in this tool.\n")
	fmt.Fprintf(os.Stderr, "  In order to proceed either the installed Miniflux must be updated, or the submodule 'sub/miniflux' of this tool.\n")
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "  Error 'Failed: cannot update entries in database: store: unable to start transaction: EOF' specifies that network connection to the database is unstable.\n")
	fmt.Fprintf(os.Stderr, "  Use parameter '-retries' to increase amount of attempts, or connect to a stable network.\n")
	fmt.Fprintf(os.Stderr, "\n")
}

func parseArgs() (Config, error) {
	var err error
	var config Config

	// Command line
	optind := permutateArgs(os.Args)

	flag.Usage = printUsage
	dbUrlOpt := flag.String("dburl", "", "(mandatory) Database connection URL, ex.: postgres://miniflux:secret@db/miniflux?sslmode=disable")
	usernameOpt := flag.String("user", "", "(mandatory) Name of the user of the entries")
	feedOpt := flag.String("feed", "", "(mandatory?) URL of the feed to assign the entries; must be specified the feed URL or the feed map file")
	feedMapOpt := flag.String("feedmap", "", "(mandatory?) Feed map file; must be specified the feed URL or the feed map file")
	markReadOpt := flag.Bool("mark", false, "Mark the inserted entries as read")
	updateOpt := flag.Bool("update", false, "Update existent entries in the database")
	removeOpt := flag.Bool("remove", false, "Remove existent entries with matched user and hash from the database")
	batchOpt := flag.Int("batch", 1000, "Pseudo-amount of messages to commit to the database at a time")
	dryOpt := flag.Bool("dry", false, "Dry run: read EML and attempt necessary transformations, but do not commit changes to the database")
	retriesOpt := flag.Int("retries", 10, "Amount of attempts to run a database transaction")
	quietOpt := flag.Bool("quiet", false, "Suppress output about unmatched messages")
	dumpOpt := flag.String("dump", "", "Write extracted EML entries dump to a specified file")
	flag.Parse()

	// Process non-options
	for _, a := range os.Args[optind:] {
		config.MessageFile = a
	}

	// Validate options
	if len(config.MessageFile) == 0 {
		return Config{}, fmt.Errorf("EML file is not specified")
	}

	// Detect message file type
	config.MessageType, err = messageFileType(config.MessageFile)
	if err != nil {
		return Config{}, err
	}

	config.DatabaseUrl = *dbUrlOpt
	if len(config.DatabaseUrl) == 0 {
		return Config{}, fmt.Errorf("database URL is not specified")
	}

	config.BatchSize = *batchOpt
	if config.BatchSize < 1 {
		return Config{}, fmt.Errorf("batch size must be positive")
	}

	config.Retries = *retriesOpt
	if config.Retries <= 0 {
		return Config{}, fmt.Errorf("retries amount must be positive")
	}

	config.Quiet = *quietOpt
	config.DumpFile = *dumpOpt
	config.MarkRead = *markReadOpt
	config.Update = *updateOpt
	config.Remove = *removeOpt
	config.DryRun = *dryOpt

	if config.DryRun && (config.Update || config.Remove) {
		fmt.Fprintf(os.Stdout, "Options '-update' and '-remove' do not have effect when '-dry' is specified.\n")
	}

	// Options required only for EML processing
	if config.MessageType == MESSAGE_EML || config.MessageType == MESSAGE_DIRECTORY {
		// Username
		config.Username = *usernameOpt
		if len(config.Username) == 0 {
			return Config{}, fmt.Errorf("user must be specified")
		}

		// Feed & FeedMap
		config.Feed = *feedOpt
		config.FeedMapFile = *feedMapOpt
		if len(config.Feed) == 0 && len(config.FeedMapFile) == 0 {
			return Config{}, fmt.Errorf("feed URL or feed map file should be specified")
		}
		if len(config.Feed) > 0 && len(config.FeedMapFile) > 0 {
			return Config{}, fmt.Errorf("feed URL and feed map file cannot be specified together")
		}
	}

	return config, nil
}

func messageFileType(filePath string) (int, error) {
	isDir, err := util.IsDirectory(filePath)
	if err != nil {
		return 0, fmt.Errorf("unable to get file info for '%s': %v", filePath, err)
	}

	if isDir {
		return MESSAGE_DIRECTORY, nil
	} else if strings.HasSuffix(strings.ToLower(filePath), ".eml") {
		return MESSAGE_EML, nil
	} else if strings.HasSuffix(strings.ToLower(filePath), ".json") {
		return MESSAGE_JSON, nil
	}

	return 0, fmt.Errorf("program argument should be a directory or file with extension '.eml' or '.json': '%s'", filePath)
}

type App struct {
	Config      Config
	DbProc      eml2miniflux.DatabaseProcessor
	user        *model.User
	feedHelper  *eml2miniflux.FeedHelper
	defaultFeed *model.Feed
}

func (a *App) init() error {
	// Required only for EML processing
	if a.Config.MessageType == MESSAGE_EML || a.Config.MessageType == MESSAGE_DIRECTORY {
		var err error

		// Get user
		a.user, err = a.DbProc.Store.UserByUsername(a.Config.Username)
		if err != nil {
			return fmt.Errorf("unable to retreive user by name '%s': %v", a.Config.Username, err)
		}

		// Feed lookup helper
		a.feedHelper, err = eml2miniflux.CreateFeedHelper(a.DbProc.Store, a.user)
		if err != nil {
			return fmt.Errorf(`cannot create feed helper: %v`, err)
		}

		// Default feed from command line
		if len(a.Config.FeedMapFile) > 0 {
			err = a.feedHelper.LoadMap(a.Config.FeedMapFile)
			if err != nil {
				return fmt.Errorf(`cannot load feed map file: %v`, err)
			}
		} else {
			a.defaultFeed = a.feedHelper.FeedByURL(a.Config.Feed)
			if a.defaultFeed == nil {
				return fmt.Errorf("unable to retrieve user by name '%s': %v", a.Config.Username, err)
			}
		}
	}

	return nil
}

func (a *App) run() error {
	entries, err := a.loadEntries()
	if err != nil {
		return fmt.Errorf("unable to load entries: %v", err)
	}

	if a.Config.MarkRead {
		for _, entry := range entries {
			entry.Status = model.EntryStatusRead
		}
	}

	err = a.dumpJson(entries)
	if err != nil {
		return err
	}

	if !a.Config.DryRun {
		err = a.removeFromDb(entries)
		if err != nil {
			return err
		}

		err = a.insertIntoDb(entries)
		if err != nil {
			return err
		}
	}

	return nil
}

func (a *App) loadEntries() (model.Entries, error) {
	var err error
	var entries model.Entries

	fmt.Fprintf(os.Stdout, "Loading entries...\n")

	switch a.Config.MessageType {
	case MESSAGE_EML, MESSAGE_DIRECTORY:
		entries, err = eml2miniflux.GetEntriesForEML(a.DbProc.Store, a.feedHelper, a.Config.MessageFile, a.user, a.defaultFeed, a.Config.Quiet)
	case MESSAGE_JSON:
		entries, err = a.loadJson()
	default:
		return nil, fmt.Errorf(`unknown message type: %d`, a.Config.MessageType)
	}

	fmt.Fprintf(os.Stdout, "Loading entries completed.\n")
	fmt.Fprintf(os.Stdout, "Loaded entries: %d\n", len(entries))

	return entries, err
}

func (a *App) loadJson() (model.Entries, error) {
	data, err := os.ReadFile(a.Config.MessageFile)
	if err != nil {
		return nil, fmt.Errorf(`cannot read message file: %v`, err)
	}

	var entries model.Entries
	err = json.Unmarshal(data, &entries)
	if err != nil {
		return nil, fmt.Errorf(`cannot unmarshal JSON: %v`, err)
	}
	return entries, nil
}

func (a *App) dumpJson(entries model.Entries) error {
	if len(a.Config.DumpFile) > 0 {
		fmt.Fprintf(os.Stdout, "Dumping to JSON...\n")
		dump, err := json.Marshal(entries)
		if err != nil {
			return fmt.Errorf(`cannot dump entries into JSON: %v`, err)
		}

		err = os.WriteFile(a.Config.DumpFile, dump, 0666)
		if err != nil {
			return fmt.Errorf(`cannot write JSON to file: %v`, err)
		}
		fmt.Fprintf(os.Stdout, "Dumping to JSON completed.\n")
	}

	return nil
}

func (a *App) removeFromDb(entries model.Entries) error {
	if a.Config.Remove {
		fmt.Fprintf(os.Stdout, "Removal from DB...\n")
		err := a.DbProc.RemoveStorageEntries(entries)
		if err == nil {
			fmt.Fprintf(os.Stdout, "Removal from DB completed.\n")
		} else {
			return fmt.Errorf(`cannot remove entries from database: %v`, err)
		}
	}

	return nil
}

func (a *App) insertIntoDb(entries model.Entries) error {
	fmt.Fprintf(os.Stdout, "Insertion into DB...\n")
	err := a.DbProc.UpdateStorageEntries(entries, a.Config.Update)

	// Statistic
	var insertedEntries int
	for _, entry := range entries {
		if entry.ID != 0 {
			insertedEntries++
		}
	}

	fmt.Fprintf(os.Stdout, "Total inserted entries: %d\n", insertedEntries)
	if err == nil {
		fmt.Fprintf(os.Stdout, "Insertion into DB completed.\n")
	} else {
		return fmt.Errorf(`cannot update entries in database: %v`, err)
	}

	return nil
}

func runApp(config Config) error {
	// Connect to DB
	db, err := database.NewConnectionPool(config.DatabaseUrl, 0, 0, 0)
	if err != nil {
		return fmt.Errorf("unable to initialize database connection pool: %v", err)
	}
	defer db.Close()

	store := storage.NewStorage(db)
	if err := store.Ping(); err != nil {
		return fmt.Errorf("unable to connect to the database: %v", err)
	}

	// Check if the used app version corresponds to the database
	if err := database.IsSchemaUpToDate(db); err != nil {
		return fmt.Errorf(`you must run the SQL migrations, %v`, err)
	}

	a := App{
		Config: config,
		DbProc: eml2miniflux.DatabaseProcessor{
			Db:        db,
			Store:     store,
			BatchSize: config.BatchSize,
			Retries:   config.Retries,
		},
	}

	err = a.init()
	if err != nil {
		return err
	}

	return a.run()
}

func mainReturn() int {
	config, err := parseArgs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Wrong command line: %s\n", err)
		printUsage()
		return 1
	}

	err = runApp(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed: %s\n", err)
		return 2
	}

	return 0
}

func main() {
	os.Exit(mainReturn())
}
