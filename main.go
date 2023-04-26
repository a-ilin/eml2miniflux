package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/a-ilin/eml2miniflux/eml2miniflux"
	"miniflux.app/database"
	"miniflux.app/model"
	"miniflux.app/storage"
)

type Config struct {
	DatabaseUrl string
	MessageFile string
	Username    string
	Feed        string
	FeedMapFile string
	BatchSize   int
	Retries     int
	DryRun      bool
}

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
	fmt.Fprintf(os.Stderr, "Usage:\n")
	fmt.Fprintf(os.Stderr, "  %s <options> <EML_file | directory>\n", prog)
	fmt.Fprintf(os.Stderr, "Options:\n")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\nExample with the feed map file:\n")
	fmt.Fprintf(os.Stderr, "  %s -dburl=postgres://miniflux:password@server:5432/miniflux?sslmode=disable -user=john -feedmap=/path/to/feed_helper.txt /path/to/rss.eml\n", prog)
	fmt.Fprintf(os.Stderr, "\nExample with the feed URL:\n")
	fmt.Fprintf(os.Stderr, "  %s -dburl=postgres://miniflux:password@server:5432/miniflux?sslmode=disable -user=john -feed=https://example.com/rss.xml /path/to/rss.eml\n", prog)
	fmt.Fprintf(os.Stderr, "\n")
}

func parseArgs() (Config, error) {
	var config Config

	// Command line
	optind := permutateArgs(os.Args)

	flag.Usage = printUsage
	dbUrlOpt := flag.String("dburl", "", "(mandatory) Database connection URL, ex.: postgres://miniflux:secret@db/miniflux?sslmode=disable")
	usernameOpt := flag.String("user", "", "(mandatory) Name of the user of the entries. Must be specified.")
	feedOpt := flag.String("feed", "", "(mandatory?) URL of the feed to assign the entries. Must be specified the feed URL or feed map file.")
	feedMapOpt := flag.String("feedmap", "", "(mandatory?) Feed map file. Must be specified the feed URL or feed map file.")
	batchOpt := flag.Int("batch", 1000, "Pseudo-amount of messages to commit to DB")
	dryOpt := flag.Bool("dry", false, "Dry run: read EML and attempt necessary transformations, but do not commit changes to the database.")
	retriesOpt := flag.Int("retries", 10, "Amount of attempts to run a database transaction")
	flag.Parse()

	// Process non-options
	for _, a := range os.Args[optind:] {
		config.MessageFile = a
	}

	// Validate options
	if len(config.MessageFile) == 0 {
		return Config{}, fmt.Errorf("EML file is not specified")
	}

	config.DatabaseUrl = *dbUrlOpt
	if len(config.DatabaseUrl) == 0 {
		return Config{}, fmt.Errorf("database URL is not specified")
	}

	config.Username = *usernameOpt
	if len(config.Username) == 0 {
		return Config{}, fmt.Errorf("user must be specified")
	}

	config.Feed = *feedOpt
	config.FeedMapFile = *feedMapOpt
	if len(config.Feed) == 0 && len(config.FeedMapFile) == 0 {
		return Config{}, fmt.Errorf("feed URL or feed map file should be specified")
	}
	if len(config.Feed) > 0 && len(config.FeedMapFile) > 0 {
		return Config{}, fmt.Errorf("feed URL and feed map file cannot be specified together")
	}

	config.BatchSize = *batchOpt
	if config.BatchSize < 1 {
		return Config{}, fmt.Errorf("batch size must be positive")
	}

	config.Retries = *retriesOpt
	if config.Retries <= 0 {
		return Config{}, fmt.Errorf("retries amount must be positive")
	}

	config.DryRun = *dryOpt

	return config, nil
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

	// Get user
	user, err := store.UserByUsername(config.Username)
	if err != nil {
		return fmt.Errorf("unable to retreive user by name '%s': %v", config.Username, err)
	}

	// Feed lookup helper
	feedHelper, err := eml2miniflux.CreateFeedHelper(store, user)
	if err != nil {
		return fmt.Errorf(`cannot create feed helper: %v`, err)
	}

	// Default feed from command line
	var defaultFeed *model.Feed
	if len(config.FeedMapFile) > 0 {
		err = feedHelper.LoadMap(config.FeedMapFile)
		if err != nil {
			return fmt.Errorf(`cannot load feed map file: %v`, err)
		}
	} else {
		defaultFeed = feedHelper.FeedByURL(config.Feed)
		if defaultFeed == nil {
			return fmt.Errorf("unable to retrieve user by name '%s': %v", config.Username, err)
		}
	}

	entries, err := eml2miniflux.GetEntriesForEML(store, feedHelper, config.MessageFile, user, defaultFeed)
	if err != nil {
		return fmt.Errorf(`cannot create entry for message: %v`, err)
	}

	if !config.DryRun {
		// Commit to DB
		fmt.Fprintf(os.Stdout, "Committing to DB...\n")
		err = eml2miniflux.UpdateStorageEntries(entries, store, config.BatchSize, config.Retries, false)
		if err == nil {
			fmt.Fprintf(os.Stdout, "Committing to DB completed.\n")
		}

		// Statistic
		var insertedEntries int
		for _, entry := range entries {
			if entry.ID != 0 {
				insertedEntries++
			}
		}
		fmt.Fprintf(os.Stdout, "Inserted entries: %d\n", insertedEntries)

		if err != nil {
			return fmt.Errorf(`cannot update entries in database: %v`, err)
		}
	}

	return nil
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
