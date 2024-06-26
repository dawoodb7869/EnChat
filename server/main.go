package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var redisClient *redis.Client

func init() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	// Initialize Redis client
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		log.Fatal("REDIS_URL is not set in .env file")
	}
	redisClient = redis.NewClient(&redis.Options{
		Addr: redisURL,
	})
}

type LastSeen struct {
	Username string      `bson:"username"`
	LastSeen interface{} `bson:"last_seen"`
}

type Message struct {
	Timestamp int64  `bson:"timestamp" json:"timestamp"`
	Content   string `bson:"content" json:"content"`
	Username  string `bson:"username" json:"username"`
}

type SystemStats struct {
	Message     string
	OS          string
	CPUModel    string
	CPUCores    int
	CPUUsage    float64
	TotalMemory float64
	FreeMemory  float64
	Uptime      string
}

func validateToken(token string) (string, error) {
	// Check if token exists in Redis
	ctx := context.Background()
	key := fmt.Sprintf("token:%s", token)
	username, err := redisClient.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			// Token not found in Redis
			return "", fmt.Errorf("invalid token")
		}
		return "", err
	}
	return username, nil
}

func getSystemStats() (SystemStats, error) {
	// Get CPU info
	cpuInfo, err := cpu.Info()
	if err != nil {
		return SystemStats{}, err
	}
	cpuUsage, err := cpu.Percent(0, false)
	if err != nil {
		return SystemStats{}, err
	}

	// Get memory info
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		return SystemStats{}, err
	}

	// Get host info
	hostStat, err := host.Info()
	if err != nil {
		return SystemStats{}, err
	}

	stats := SystemStats{
		Message:     "EnChat is running",
		OS:          runtime.GOOS,
		CPUModel:    cpuInfo[0].ModelName,
		CPUCores:    runtime.NumCPU(),
		CPUUsage:    round(cpuUsage[0], 2),
		TotalMemory: bytesToGigabytes(vmStat.Total),
		FreeMemory:  bytesToGigabytes(vmStat.Free),
		Uptime:      formatUptime(hostStat.Uptime),
	}

	return stats, nil
}

func round(value float64, precision int) float64 {
	p := float64(precision)
	shift := float64(10 * p)
	return float64(int(value*shift)) / shift
}

func bytesToGigabytes(bytes uint64) float64 {
	return round(float64(bytes)/(1024*1024*1024), 2)
}

func formatUptime(seconds uint64) string {
	duration := time.Duration(seconds) * time.Second
	days := duration / (24 * time.Hour)
	duration -= days * 24 * time.Hour
	hours := duration / time.Hour
	duration -= hours * time.Hour
	minutes := duration / time.Minute
	return fmt.Sprintf("%d days, %d hours, %d minutes", days, hours, minutes)
}

func handler(w http.ResponseWriter, r *http.Request) {
	stats, err := getSystemStats()
	if err != nil {
		http.Error(w, "Could not get system stats", http.StatusInternalServerError)
		return
	}

	tmpl, err := template.New("index").Parse(`
	<!DOCTYPE html>
	<html lang="en">
	<head>
		<meta charset="UTF-8">
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
		<title>System Stats</title>
		<style>
			body { font-family: Arial, sans-serif; margin: 40px; padding: 20px; background-color: #f4f4f4; }
			h1 { color: #333; }
			table { width: 100%; border-collapse: collapse; margin: 20px 0; }
			th, td { padding: 12px; border: 1px solid #ddd; text-align: left; }
			th { background-color: #f2f2f2; }
			footer { margin-top: 40px; font-size: 0.9em; color: #666; }
		</style>
	</head>
	<body>
		<h1>{{.Message}}</h1>
		<table>
			<tr><th>Operating System</th><td>{{.OS}}</td></tr>
			<tr><th>CPU Model</th><td>{{.CPUModel}}</td></tr>
			<tr><th>CPU Cores</th><td>{{.CPUCores}}</td></tr>
			<tr><th>CPU Usage</th><td>{{.CPUUsage}}%</td></tr>
			<tr><th>Total Memory</th><td>{{.TotalMemory}} GB</td></tr>
			<tr><th>Free Memory</th><td>{{.FreeMemory}} GB</td></tr>
			<tr><th>Uptime</th><td>{{.Uptime}}</td></tr>
		</table>
		<footer>
			<p>Version 1.0.0</p>
			<p>Developed with <span style="color: red;">&hearts;</span> by <a href="https://dawood.tech" target="_blank">Muhammad Dawood</a></p>
		</footer>
	</body>
	</html>
	`)
	if err != nil {
		http.Error(w, "Could not parse template", http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, stats); err != nil {
		http.Error(w, "Could not execute template", http.StatusInternalServerError)
	}
}

func authHandler(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")

	// Hash the provided password
	hashedPassword := hashPassword(password)

	// Authenticate user with hashed password
	if authenticateUser(username, hashedPassword) {
		// Generate a token
		token := generateToken(username)

		// Store token in Redis with a TTL of 1 day
		if err := setTokenInRedis(token, username); err != nil {
			http.Error(w, "Failed to generate token", http.StatusInternalServerError)
			return
		}

		// Return JSON response with token
		response := struct {
			Response string `json:"response"`
			Token    string `json:"token"`
		}{
			Response: "ok",
			Token:    token,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	} else {
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
	}
}

func fetchMessagesHandler(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Token not provided", http.StatusBadRequest)
		log.Println("Token not provided")
		return
	}

	// Validate token and get username
	username, err := validateToken(token)
	if err != nil {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		log.Printf("Invalid token: %v", err)
		return
	}

	if username == "" {
		http.Error(w, "Username not found for the provided token", http.StatusUnauthorized)
		log.Println("Username not found for the provided token")
		return
	}

	// Connect to MongoDB
	mongoURI := os.Getenv("MONGO_URI")
	clientOptions := options.Client().ApplyURI(mongoURI)
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		http.Error(w, "Failed to connect to MongoDB", http.StatusInternalServerError)
		log.Printf("Failed to connect to MongoDB: %v", err)
		return
	}
	defer client.Disconnect(context.Background())

	// Get last_seen collection
	lastSeenCollection := client.Database("en_chat").Collection("last_seen")

	// Get the last_seen timestamp for the user
	var lastSeenDoc struct {
		LastSeen int64 `bson:"last_seen"`
	}
	filter := bson.M{"username": username}
	err = lastSeenCollection.FindOne(context.Background(), filter).Decode(&lastSeenDoc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "No last_seen data found for the user", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to fetch last_seen data", http.StatusInternalServerError)
		}
		log.Printf("Failed to fetch last_seen data for username %s: %v", username, err)
		return
	}

	// Get messages collection
	messagesCollection := client.Database("en_chat").Collection("messages")

	// Query for messages
	cursor, err := messagesCollection.Find(context.Background(), bson.M{})
	if err != nil {
		http.Error(w, "Failed to query messages", http.StatusInternalServerError)
		log.Printf("Failed to query messages: %v", err)
		return
	}
	defer cursor.Close(context.Background())

	// Separate messages into read and unread
	var messages []Message
	var unreadMessages []Message
	for cursor.Next(context.Background()) {
		var message Message
		if err := cursor.Decode(&message); err != nil {
			http.Error(w, "Failed to decode message data", http.StatusInternalServerError)
			log.Printf("Failed to decode message data: %v", err)
			return
		}
		if message.Timestamp <= lastSeenDoc.LastSeen {
			messages = append(messages, message)
		} else {
			unreadMessages = append(unreadMessages, message)
		}
	}

	if err := cursor.Err(); err != nil {
		http.Error(w, "Error iterating through messages", http.StatusInternalServerError)
		log.Printf("Error iterating through messages: %v", err)
		return
	}

	// Return JSON response with messages and unread_messages
	response := struct {
		Messages       []Message `json:"messages"`
		UnreadMessages []Message `json:"unread_messages"`
	}{
		Messages:       messages,
		UnreadMessages: unreadMessages,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		log.Printf("Failed to encode response: %v", err)
	}
}

// Function to hash password using SHA256
func hashPassword(password string) string {
	hasher := sha256.New()
	hasher.Write([]byte(password))
	return hex.EncodeToString(hasher.Sum(nil))
}

// Function to authenticate user against MongoDB
func authenticateUser(username, hashedPassword string) bool {
	// Connect to MongoDB
	mongoURI := os.Getenv("MONGO_URI")
	clientOptions := options.Client().ApplyURI(mongoURI)
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		log.Printf("Failed to connect to MongoDB: %v\n", err)
		return false
	}
	defer client.Disconnect(context.Background())

	// Get user collection
	collection := client.Database("en_chat").Collection("users")

	// Check if user exists with matching username and hashed password
	filter := bson.M{"username": username, "password": hashedPassword}
	var result bson.M
	err = collection.FindOne(context.Background(), filter).Decode(&result)
	if err != nil {
		log.Printf("User authentication failed: %v\n", err)
		return false
	}

	// User authenticated successfully
	return true
}

func validateTokenHandler(w http.ResponseWriter, r *http.Request) {
	// Parse token from request query parameter
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Token not provided", http.StatusBadRequest)
		return
	}

	// Check if token exists in Redis
	ctx := context.Background()
	key := fmt.Sprintf("token:%s", token)
	_, err := redisClient.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			// Token not found in Redis
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}
		http.Error(w, "Failed to validate token", http.StatusInternalServerError)
		return
	}

	// Calculate remaining time for token expiration
	ttl, err := redisClient.TTL(ctx, key).Result()
	if err != nil {
		http.Error(w, "Failed to get remaining time for token", http.StatusInternalServerError)
		return
	}

	// Return JSON response with validation result and remaining time
	response := struct {
		Valid         bool          `json:"valid"`
		RemainingTime time.Duration `json:"remaining_time"`
	}{
		Valid:         true,
		RemainingTime: ttl,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func generateToken(username string) string {
	// Generate token based on username and current timestamp
	currentTime := time.Now().UnixNano()
	tokenData := fmt.Sprintf("%s%d", username, currentTime)
	hasher := sha256.New()
	hasher.Write([]byte(tokenData))
	return hex.EncodeToString(hasher.Sum(nil))
}

func setTokenInRedis(token, username string) error {
	// Set token in Redis with a TTL of 1 day
	ctx := context.Background()
	key := fmt.Sprintf("token:%s", token)
	return redisClient.Set(ctx, key, username, 24*time.Hour).Err()
}

func onlineUsersHandler(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Token not provided", http.StatusBadRequest)
		return
	}

	// Validate token
	_, err := validateToken(token)
	if err != nil {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	// Connect to MongoDB
	mongoURI := os.Getenv("MONGO_URI")
	clientOptions := options.Client().ApplyURI(mongoURI)
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		http.Error(w, "Failed to connect to MongoDB", http.StatusInternalServerError)
		return
	}
	defer client.Disconnect(context.Background())

	// Get last_seen collection
	collection := client.Database("en_chat").Collection("last_seen")

	// Query for users who have a last_seen status of "online"
	filter := bson.M{"last_seen": "online"}
	cursor, err := collection.Find(context.Background(), filter)
	if err != nil {
		http.Error(w, "Failed to query online users", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.Background())

	// Collect the usernames of online users
	var onlineUsers []string
	for cursor.Next(context.Background()) {
		var user LastSeen
		if err := cursor.Decode(&user); err != nil {
			http.Error(w, "Failed to decode user data", http.StatusInternalServerError)
			return
		}
		onlineUsers = append(onlineUsers, user.Username)
	}

	// Return the list of online users as JSON
	w.Header().Set("Content-Type", "application/json")
	if len(onlineUsers) == 0 {
		json.NewEncoder(w).Encode([]string{})
	} else {
		json.NewEncoder(w).Encode(onlineUsers)
	}
}

func main() {
	http.HandleFunc("/", handler)
	http.HandleFunc("/auth", authHandler)
	http.HandleFunc("/validate_token", validateTokenHandler)
	http.HandleFunc("/online_users", onlineUsersHandler)
	http.HandleFunc("/fetch_messages", fetchMessagesHandler)

	fmt.Println("Server is running at http://localhost:8080/")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Printf("Failed to start server: %v\n", err)
	}
}
