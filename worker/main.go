package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

type Vote struct {
	VoterID string `json:"voter_id"`
	Vote    string `json:"vote"`
}

const healthyFilePath = "/tmp/healthy"

func main() {
	// flag cli -healthcheck (false by default) for docker healthcheck
    healthcheckFlag := flag.Bool("healthcheck", false, "Lancer la sonde de santé")
    flag.Parse()

        
    if *healthcheckFlag {
        runHealthcheck()
        return
    }
	runWorker()
}

func runHealthcheck() {
	// check the fille in /tmp/healthy updated by updateHeartbeat()
	info, err := os.Stat(healthyFilePath)
        if err != nil {
            fmt.Fprintf(os.Stderr, "Healthcheck failed : %v\n", err) // the hearbit file does not exist (1st healthcheck cannot check time since last heartbeat)
            os.Exit(1)
        }
    
        if time.Since(info.ModTime()) > 16*time.Second { 
            fmt.Fprintf(os.Stderr, "Healthcheck failed : too old (%v)\n", time.Since(info.ModTime()))
            os.Exit(1)
        }

        fmt.Println("Healthcheck OK")
        os.Exit(0)
}


func updateHeartbeat() {
    data := []byte(time.Now().UTC().Format(time.RFC3339)) // write YYYY-MM-DDTHH:MM:UTC in /tmp/healthy
    _ = os.WriteFile(healthyFilePath, data, 0644)
    
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
            return value
        }
        return fallback
}

func getDbPassword() (string, error) {
        if passwdFile := os.Getenv("POSTGRES_PASSWORD_FILE"); passwdFile != "" {
            data, err := os.ReadFile(passwdFile)
            if err != nil {
                return "", fmt.Errorf("Impossible to read secret file %s: %w", passwdFile, err)
            }
            return strings.TrimSpace(string(data)), nil
        }

        // no secret file -> error
        return "", fmt.Errorf("No password provided. Please set POSTGRES_PASSWORD_FILE (db_password secret file)") 
    }

func openDbConnection(connStr string) *sql.DB {
	for {
		db, err := sql.Open("postgres", connStr)
		if err == nil {
			// Ping db to check connection
			if err = db.Ping(); err == nil {
				log.Println("Connected to the database")

				query := `CREATE TABLE IF NOT EXISTS votes (
                        id VARCHAR(255) NOT NULL UNIQUE,
                        vote VARCHAR(255) NOT NULL
                    )`

					_ , createQueryErr := db.Exec(query) // vs db.Query bc we don't need any returned rows
                    if createQueryErr != nil {
                        log.Printf("Error creating table : %v\n", createQueryErr)
                    } else {
                        return db 
                    }
                }
            }

            log.Println("Waiting for db")
            time.Sleep(1 * time.Second)
	}
}

func openRedisClient(host, port string) *redis.Client {
	addr := net.JoinHostPort(host, port)
	for {
		log.Printf("Connecting to redis at %s\n", addr)
		rdb := redis.NewClient(&redis.Options{ Addr: addr, DB: 0 })
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
            err := rdb.Ping(ctx).Err()
            cancel()
		

		if err == nil {
                log.Println("Connected to redis")
                return rdb 
            }

		log.Println("Waiting for redis")
        time.Sleep(1 * time.Second)
	}
}

func updateVote(db *sql.DB, voterID, vote string) error {
	query := `INSERT INTO votes (id, vote) VALUES ($1, $2)
			  ON CONFLICT (id) DO UPDATE SET vote = EXCLUDED.vote`
	_, err := db.Exec(query, voterID, vote)
	return err
}


func runWorker() {
	pgHost := getEnv("POSTGRES_HOST", "db")
    pgPort := getEnv("POSTGRES_PORT", "5432")
    pgUser := getEnv("POSTGRES_USER", "postgres")
    pgDb   := getEnv("POSTGRES_DB", "postgres")

    redisHost := getEnv("REDIS_HOST", "redis")
    redisPort := getEnv("REDIS_PORT", "6379")

	pgPass, err := getDbPassword()
    if err != nil {
        log.Fatalf("Error in fatal configuration : %v", err)
        // show err message and exit
    }

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
        pgHost, pgPort, pgUser, pgPass, pgDb)

	db := openDbConnection(connStr)
    defer db.Close()  // close db connection when main() exits
    rdb := openRedisClient(redisHost, redisPort)
    defer rdb.Close() // close redis connection 

	ctx := context.Background()
    
    log.Println("Worker is running and waiting for votes")

	for {
		time.Sleep(100 * time.Millisecond)
		updateHeartbeat()

		voteData, err := rdb.LPop(ctx, "votes").Result()
		if err == redis.Nil {
			_ = db.Ping()
		} else if err != nil {
			log.Printf("Error occurred while popping from redis: %v", err)
		} else {
			var vote Vote
            if err := json.Unmarshal([]byte(voteData), &vote); err != nil {
                log.Printf("Error decoding JSON: %v\n", err)
                continue
            }
			log.Printf("Processing vote for '%s' by '%s'\n", vote.Vote, vote.VoterID)
			if err := updateVote(db, vote.VoterID, vote.Vote); err != nil {
                log.Printf("Error updating vote in DB: %v\n", err)
            }
		}
	}
}
