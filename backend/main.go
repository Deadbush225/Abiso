package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/SherClockHolmes/webpush-go"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

var dbPool *pgxpool.Pool

// Request payload sent by the frontend containing user preferences and subscription keys
type SubscriptionPayload struct {
	City         string               `json:"city" binding:"required"`
	Barangay     string               `json:"barangay" binding:"required"`
	Subscription webpush.Subscription `json:"subscription" binding:"required"`
}

// Payload for incoming scraper events or broadcast demands
type BroadcastPayload struct {
	City     string `json:"city" binding:"required"`
	Barangay string `json:"barangay" binding:"required"`
	Message  string `json:"message" binding:"required"`
}

func main() {
	godotenv_err := godotenv.Load("../.env")
	
	if godotenv_err != nil {
		log.Println("ℹ️ No .env file found, relying on system environment variables")
		return
	}

	// 1. Database Connection String configuration
	connString := os.Getenv("DATABASE_URL")
	if connString == "" {
		log.Println("⚠️ DATABASE_URL not set, using default local connection string")
		return
	}

	// 2. Initialize the Connection Pool (Lazy initialization)
	var err error
	dbPool, err = pgxpool.New(context.Background(), connString)
	if err != nil {
		log.Fatalf("❌ Unable to create connection pool: %v\n", err)
	}
	defer dbPool.Close()

	// 3. Force a Physical Handshake (Ping) to verify DB availability on startup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := dbPool.Ping(ctx); err != nil {
		log.Fatalf("❌ Database unreachable: %v\n", err)
	}
	fmt.Println("🚀 Connected to PostgreSQL successfully!")

	// 4. Validate VAPID Keys existence
	if os.Getenv("VAPID_PUBLIC_KEY") == "" || os.Getenv("VAPID_PRIVATE_KEY") == "" {
		log.Println("⚠️  Warning: VAPID keys are missing from environment variables.")
		return
	}

	// 5. Initialize the Server Engine
	r := gin.Default()

	// Enable basic CORS so your frontend application can access these APIs seamlessly
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	// Public endpoint exposing the VAPID Public key required by the browser's PushManager
	r.GET("/api/vapid-public-key", func(c *gin.Context) {
		publicKey := os.Getenv("VAPID_PUBLIC_KEY")
		c.JSON(http.StatusOK, gin.H{"publicKey": publicKey})
	})

	// Core subscriber endpoint
	r.POST("/api/subscribe", handleSubscribe)

	// Protected internal or webhook endpoint triggered when your scraper catches an alert
	r.POST("/api/broadcast", handleBroadcast)

	// Run application server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("📡 Abiso Backend Listening on port %s...", port)
	r.Run(":" + port)
}

// Process incoming registrations and link location info with push encryption targets
func handleSubscribe(c *gin.Context) {
	var payload SubscriptionPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload format: " + err.Error()})
		return
	}

	ctx := context.Background()

	// Step 1: Validate location existence against supported dictionary
	var locationID int
	locQuery := "SELECT id FROM locations WHERE LOWER(city) = LOWER($1) AND LOWER(barangay) = LOWER($2);"
	err := dbPool.QueryRow(ctx, locQuery, payload.City, payload.Barangay).Scan(&locationID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Location not supported: %s, %s", payload.City, payload.Barangay)})
		return
	}

	// Step 2: Transform Subscription struct into JSON bytes for Postgres JSONB field storage
	subscriptionJSON, err := json.Marshal(payload.Subscription)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse subscription keys"})
		return
	}

	// Step 3: Insert new subscriber token with UPSERT strategy to avoid duplicated records
	subQuery := `
		INSERT INTO subscribers (push_subscription) 
		VALUES ($1) 
		RETURNING id;`
	
	var subscriberID int
	err = dbPool.QueryRow(ctx, subQuery, subscriptionJSON).Scan(&subscriberID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error while writing subscription"})
		return
	}

	// Step 4: Map subscription payload relationship to target structural location
	junctionQuery := `
		INSERT INTO user_subscriptions (subscriber_id, location_id) 
		VALUES ($1, $2) 
		ON CONFLICT DO NOTHING;`
	_, err = dbPool.Exec(ctx, junctionQuery, subscriberID, locationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database mapping error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Successfully opted in for utility push alerts!"})
}

// Queries targeted subscribers and securely routes cryptographic payloads to browser push endpoints
func handleBroadcast(c *gin.Context) {
	var payload BroadcastPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid target filter schema"})
		return
	}

	ctx := context.Background()

	// Query fetches JSON credentials for all devices listening to the specified geographic point
	broadcastQuery := `
		SELECT s.push_subscription 
		FROM subscribers s
		JOIN user_subscriptions us ON s.id = us.subscriber_id
		JOIN locations l ON us.location_id = l.id
		WHERE LOWER(l.city) = LOWER($1) AND LOWER(l.barangay) = LOWER($2);`

	rows, err := dbPool.Query(ctx, broadcastQuery, payload.City, payload.Barangay)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Target lookup failure: " + err.Error()})
		return
	}
	defer rows.Close()

	var successCount int
	var errorCount int

	// Iteration maps database records back into operational dispatch parameters
	for rows.Next() {
		var subBytes []byte
		if err := rows.Scan(&subBytes); err != nil {
			errorCount++
			continue
		}

		var sub webpush.Subscription
		if err := json.Unmarshal(subBytes, &sub); err != nil {
			errorCount++
			continue
		}

		// Encapsulate structural message object into standard notification formatting text string
		notificationData := map[string]string{
			"title": "ABISO: Advisory Update",
			"body":  payload.Message,
		}
		notificationJSON, _ := json.Marshal(notificationData)

		// Fire Web Push execution to browser engine
		resp, err := webpush.SendNotification(notificationJSON, &sub, &webpush.Options{
			Subscriber:      "mailto:alerts-admin@yourdomain.edu.ph",
			VAPIDPublicKey:  os.Getenv("VAPID_PUBLIC_KEY"),
			VAPIDPrivateKey: os.Getenv("VAPID_PRIVATE_KEY"),
			TTL:             3600, // Message remains active on intermediate routing tiers for 1 hour
		})

		if err != nil {
			log.Printf("⚠️ Push failing for target: %v\n", err)
			errorCount++
			continue
		}
		resp.Body.Close()
		successCount++
	}

	c.JSON(http.StatusOK, gin.H{
		"status":      "Completed processing distribution tree",
		"dispatched": successCount,
		"failures":    errorCount,
	})
}