package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/confuzeus/replyforge/internal/config"
	"github.com/confuzeus/replyforge/internal/model"
	"github.com/confuzeus/replyforge/internal/repository"
	"github.com/confuzeus/replyforge/internal/sanitizer"
	"github.com/confuzeus/replyforge/migrations"

	_ "github.com/mattn/go-sqlite3"
)

var (
	postIDs = []string{
		"hello-world",
		"getting-started-with-go",
		"building-rest-apis",
		"sqlite-performance-tips",
		"concurrency-patterns",
	}

	authorNames = []string{
		"Sarah Chen",
		"Marcus Johnson",
		"Alex Rivera",
		"Priya Patel",
		"James Wilson",
		"Emily Thompson",
		"Carlos Mendez",
		"Aisha Rahman",
		"David Kim",
		"Lena Fischer",
		"Omar Hassan",
		"Yuki Tanaka",
		"Michael O'Brien",
		"Sofia Rossi",
		"Rajesh Kumar",
		"Emma Johansson",
		"Wei Zhang",
		"Fatima Al-Sayed",
		"Tomás García",
		"Nina Petrova",
	}

	bodies = []string{
		"Great article! Really helped me understand the topic.",
		"I've been using this approach for years and it works great.",
		"Could you elaborate on the error handling section?",
		"Thanks for sharing! I learned something new today.",
		"This is exactly what I was looking for. Bookmarked!",
		"Would love to see a follow-up post on production deployment.",
		"Clear and concise. More articles should be written like this.",
		"I tried this approach and ran into an issue with concurrent writes. Any advice?",
		"Does this work with the latest version of the framework?",
		"Excellent write-up. The diagrams really helped visualize the architecture.",
		"I've shared this with my team. Very practical advice.",
		"One question: how does this handle edge cases with empty inputs?",
		"This changed my perspective on the problem. Thanks!",
		"Very detailed tutorial. I appreciate the step-by-step breakdown.",
		"Not sure I agree with the conclusion, but interesting read nonetheless.",
		"Finally someone explained this in a way that makes sense!",
		"Could you also cover testing strategies for this pattern?",
		"I wish I had found this article weeks ago. Would have saved me hours.",
		"Good introduction, but I think it could benefit from more real-world examples.",
		"Straight to the point. Refreshing to see on the internet!",
	}

	ips = []string{
		"192.168.1.100",
		"10.0.0.42",
		"203.0.113.5",
		"198.51.100.20",
		"172.16.0.50",
		"185.220.101.33",
		"91.198.174.192",
		"45.33.32.156",
		"104.16.132.229",
		"8.8.8.8",
	}

	userAgents = []string{
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Mobile/15E148 Safari/604.1",
		"Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.6099.144 Mobile Safari/537.36",
		"curl/8.4.0",
	}
)

func main() {
	var count int
	var dbPath string
	var clear bool

	flag.IntVar(&count, "count", 50, "Number of comments to generate")
	flag.IntVar(&count, "n", 50, "Number of comments to generate (shorthand)")
	flag.StringVar(&dbPath, "db", "", "Database path (overrides DATABASE_PATH env)")
	flag.BoolVar(&clear, "clear", false, "Clear all existing comments before seeding")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: seed [options]\n\n")
		fmt.Fprintf(os.Stderr, "Seed the database with realistic comments for development.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	cfg := config.Load()
	databasePath := cfg.DatabasePath
	if dbPath != "" {
		databasePath = dbPath
	}

	dsn := databasePath + "?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on"

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error opening database:", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		fmt.Fprintln(os.Stderr, "error connecting to database:", err)
		os.Exit(1)
	}

	if err := config.RunMigrations(db, migrations.SQL); err != nil {
		fmt.Fprintln(os.Stderr, "error running migrations:", err)
		os.Exit(1)
	}

	ctx := context.Background()

	if clear {
		fmt.Print("Clearing existing comments... ")
		if _, err := db.ExecContext(ctx, "DELETE FROM comments"); err != nil {
			fmt.Fprintln(os.Stderr, "error clearing comments:", err)
			os.Exit(1)
		}
		if _, err := db.ExecContext(ctx, "DELETE FROM sqlite_sequence WHERE name='comments'"); err != nil {
			fmt.Fprintln(os.Stderr, "error resetting auto-increment:", err)
		}
		fmt.Println("done")
	}

	repo := repository.NewCommentRepository(db)
	displayIDGen := model.NewDisplayIDGenerator()
	san := sanitizer.NewSanitizer()
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	start := time.Now()

	for i := range count {
		createdAt := time.Now().Add(-time.Duration(rng.Intn(720)) * time.Hour)
		createdAtStr := createdAt.UTC().Format(time.RFC3339)

		comment := &repository.Comment{
			PostID:            postIDs[rng.Intn(len(postIDs))],
			AuthorName:        san.Sanitize(authorNames[rng.Intn(len(authorNames))]),
			Body:              san.Sanitize(bodies[rng.Intn(len(bodies))]),
			Approved:          rng.Float64() < 0.8,
			IPAddress:         ips[rng.Intn(len(ips))],
			UserAgent:         userAgents[rng.Intn(len(userAgents))],
			TurnstileVerified: rng.Float64() < 0.9,
		}

		id, err := repo.Insert(ctx, comment)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nerror inserting comment %d: %v\n", i+1, err)
			continue
		}

		displayID, err := displayIDGen.Generate(id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nerror generating display ID for comment %d: %v\n", id, err)
			continue
		}

		if _, err := db.ExecContext(ctx,
			`UPDATE comments SET display_id = ?, created_at = ?, updated_at = ? WHERE id = ?`,
			displayID, createdAtStr, createdAtStr, id,
		); err != nil {
			fmt.Fprintf(os.Stderr, "\nerror updating comment %d: %v\n", id, err)
		}
	}

	elapsed := time.Since(start)
	fmt.Printf("Seeded %d comments in %s\n", count, elapsed.Round(time.Millisecond))
}
