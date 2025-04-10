package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	jira "github.com/andygrunwald/go-jira/v2/onpremise"
	"github.com/spf13/viper"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Config struct {
	Jira struct {
		URL   string `mapstructure:"url"`
		Token string `mapstructure:"token"`
	} `mapstructure:"jira"`
	MongoDB struct {
		URI        string `mapstructure:"uri"`
		Database   string `mapstructure:"database"`
		Collection string `mapstructure:"collection"`
	} `mapstructure:"mongodb"`
	Server struct {
		Port int `mapstructure:"port"`
	} `mapstructure:"server"`
}

func loadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Set default values if not provided
	if config.Server.Port == 0 {
		config.Server.Port = 8080
	}

	return &config, nil
}

func createJiraClientWithBackoff(config *Config) (*jira.Client, error) {
	jiraURL := config.Jira.URL

	// Create PAT auth transport with backoff
	tp := jira.PATAuthTransport{
		Token: config.Jira.Token,
		Transport: &backoffTransport{
			transport:      http.DefaultTransport,
			maxRetries:     5,
			initialBackoff: 1 * time.Second,
		},
	}

	// Create Jira client with custom HTTP client
	jiraClient, err := jira.NewClient(jiraURL, tp.Client())
	if err != nil {
		return nil, fmt.Errorf("error creating Jira client: %w", err)
	}

	return jiraClient, nil
}

// Backoff transport implementation
type backoffTransport struct {
	transport      http.RoundTripper
	maxRetries     int
	initialBackoff time.Duration
}

func (t *backoffTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	backoff := t.initialBackoff

	for retries := 0; retries <= t.maxRetries; retries++ {
		// Clone request as it can't be reused
		reqCopy := req.Clone(req.Context())

		resp, err = t.transport.RoundTrip(reqCopy)
		if err != nil {
			return nil, err
		}

		// If not rate limited or last retry, return response
		if resp.StatusCode != 429 || retries == t.maxRetries {
			return resp, nil
		}

		// Get Retry-After header or use exponential backoff
		retryAfter := 0
		if retryAfterStr := resp.Header.Get("Retry-After"); retryAfterStr != "" {
			if retryAfterSec, parseErr := strconv.Atoi(retryAfterStr); parseErr == nil {
				retryAfter = retryAfterSec
			}
		}

		// Close response before retrying
		resp.Body.Close()

		// Calculate backoff duration
		var sleepDuration time.Duration
		if retryAfter > 0 {
			sleepDuration = time.Duration(retryAfter) * time.Second
		} else {
			sleepDuration = backoff
			backoff *= 2 // Exponential backoff
		}

		// Log the backoff
		log.Printf("Rate limited by Jira API. Retrying in %v. Attempt %d/%d",
			sleepDuration, retries+1, t.maxRetries)

		select {
		case <-time.After(sleepDuration):
			// Continue with retry
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	}

	// This should never be reached due to the return conditions in the loop
	return resp, err
}

func createMongoClient(config *Config) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(config.MongoDB.URI))
	if err != nil {
		return nil, fmt.Errorf("error connecting to MongoDB: %w", err)
	}

	return client, nil
}

type TargetVersion struct {
	Name string `structs:"name"`
}

const maxRecursionDepth = 6

func getCustomFieldValue(issue *jira.Issue, customFieldID string) string {
	if issue.Fields.Unknowns != nil {
		values := []string{}
		fieldData := fmt.Sprintf("%v", issue.Fields.Unknowns[customFieldID])
		fieldData = strings.TrimPrefix(fieldData, "[")
		fieldData = strings.TrimSuffix(fieldData, "]")
		maps := strings.Split(fieldData, "map[")
		for _, m := range maps {
			m = strings.TrimSuffix(m, "]")
			kvs := strings.Split(m, " ")
			for _, kv := range kvs {
				if strings.Contains(kv, "name") {
					kv = strings.TrimPrefix(kv, "name:")
					kv = strings.TrimSpace(kv)
					values = append(values, kv)
				}
			}
		}
		sort.Slice(values, func(i, j int) bool {
			return values[i] > values[j]
		})
		return strings.Join(values, ", ")
	}
	return ""
}

func storeIssue(jiraClient *jira.Client, issueID string, depth uint) (bson.M, error) {
	if depth > maxRecursionDepth {
		log.Printf("Max recursion depth reached for issue %s", issueID)
		return nil, nil
	}
	// Get the issue from Jira
	fieldFilter := "summary,status,assignee,customfield_12319940,customfield_12323940"
	if depth > 0 {
		fieldFilter = "status,customfield_12319940"
	}
	getOptions := &jira.GetQueryOptions{
		Fields: fieldFilter,
	}
	issue, _, err := jiraClient.Issue.Get(context.Background(), issueID, getOptions)
	if err != nil {
		return nil, fmt.Errorf("error getting issue %s: %w", issueID, err)
	}

	// Prepare data for MongoDB
	data := bson.M{
		"_id": issue.Key,
	}
	if issue.Fields.Status != nil {
		data["status"] = issue.Fields.Status.Name
	}
	data["target_version"] = getCustomFieldValue(issue, "customfield_12319940")
	if depth == 0 {
		if issue.Fields.Assignee != nil {
			data["assignee"] = issue.Fields.Assignee.DisplayName
		}
		data["summary"] = issue.Fields.Summary
		data["target_backport_versions"] = getCustomFieldValue(issue, "customfield_12323940")
	}

	// use curl -H "Authorization: Bearer <token>" https://issues.redhat.com/rest/api/latest/field
	// to get the custom field IDs

	// Search for issues that are clones of the given issue
	jql := fmt.Sprintf(`issue in linkedIssues("%s", "is cloned by") ORDER BY created DESC`, issue.Key)

	searchOptions := &jira.SearchOptions{
		Fields:     []string{"status"},
		MaxResults: 1,
	}
	clones, _, err := jiraClient.Issue.Search(context.Background(), jql, searchOptions)
	if err != nil {
		return nil, fmt.Errorf("error searching for cloned issues: %w", err)
	}
	if len(clones) > 0 {
		clone := clones[0]
		data["clone"], err = storeIssue(jiraClient, clone.Key, depth+1)
		if err != nil {
			return nil, fmt.Errorf("error storing cloned issue %s: %w", clone.Key, err)
		}
	}

	return data, nil
}

func getMapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func syncIssues(jiraClient *jira.Client, mongoClient *mongo.Client, config *Config) error {
	// Get MongoDB collection
	collection := mongoClient.Database(config.MongoDB.Database).Collection(config.MongoDB.Collection)

	// Keep track of documents updated in this sync
	updatedDocuments := make(map[string]bool)

	// JQL query to get main issues
	jql := "project = OCPBUGS AND component = Hypershift AND \"Target Version\" = 4.19.0 AND \"Target Backport Versions\" is not EMPTY"

	// Search issues in Jira
	searchOptions := &jira.SearchOptions{
		MaxResults: 50,
		StartAt:    0,
		Fields:     []string{"status"},
	}

	for {
		issues, _, err := jiraClient.Issue.Search(context.Background(), jql, searchOptions)
		if err != nil {
			return fmt.Errorf("error searching Jira issues: %w", err)
		}

		// Process each issue
		for _, issue := range issues {
			// Extract issue data recursively from the issue and its chain of clones
			data, err := storeIssue(jiraClient, issue.Key, 0)
			if err != nil {
				log.Printf("Error storing issue %s: %v", issue.Key, err)
				continue
			}

			// Upsert to MongoDB
			upsertOpts := options.Update().SetUpsert(true)
			filter := bson.M{"_id": issue.Key}
			update := bson.M{"$set": data}

			_, err = collection.UpdateOne(context.Background(), filter, update, upsertOpts)
			if err != nil {
				log.Printf("Error upserting issue %s: %v", issue.Key, err)
				continue
			}
			log.Printf("Upserted issue %s", issue.Key)

			// Track that this document was updated
			updatedDocuments[issue.Key] = true
		}

		// Check if there are more issues to fetch
		if len(issues) < searchOptions.MaxResults {
			break
		}
		searchOptions.StartAt += len(issues)
	}

	// Remove documents that were not updated in this sync
	deleteFilter := bson.M{
		"_id": bson.M{"$nin": getMapKeys(updatedDocuments)},
	}

	deleteResult, err := collection.DeleteMany(context.Background(), deleteFilter)
	if err != nil {
		return fmt.Errorf("error removing stale documents: %w", err)
	}

	log.Printf("Removed %d stale documents", deleteResult.DeletedCount)

	return nil
}

// HTTP handler functions
func getDocumentsHandler(mongoClient *mongo.Client, config *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set headers for CORS and JSON
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		// Get MongoDB collection
		collection := mongoClient.Database(config.MongoDB.Database).Collection(config.MongoDB.Collection)

		// Create context with timeout
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// Find all documents
		cursor, err := collection.Find(ctx, bson.M{})
		if err != nil {
			log.Printf("Error finding documents: %v", err)
			http.Error(w, "Error retrieving documents", http.StatusInternalServerError)
			return
		}
		defer cursor.Close(ctx)

		// Decode documents
		var documents []bson.M
		if err := cursor.All(ctx, &documents); err != nil {
			log.Printf("Error decoding documents: %v", err)
			http.Error(w, "Error processing documents", http.StatusInternalServerError)
			return
		}

		// Return JSON response
		if err := json.NewEncoder(w).Encode(documents); err != nil {
			log.Printf("Error encoding JSON response: %v", err)
			http.Error(w, "Error generating response", http.StatusInternalServerError)
			return
		}
	}
}

func markDocumentCompleteHandler(mongoClient *mongo.Client, config *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set headers for CORS and JSON
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		// Check if this is a preflight OPTIONS request
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusOK)
			return
		}

		// Handle only POST requests
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse request body
		var req struct {
			ID        string `json:"id"`
			Completed bool   `json:"completed"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("Error decoding request: %v", err)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		// Get MongoDB collection
		collection := mongoClient.Database(config.MongoDB.Database).Collection(config.MongoDB.Collection)

		// Create context with timeout
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// Update document
		filter := bson.M{"_id": req.ID}
		update := bson.M{"$set": bson.M{"completed": req.Completed}}

		result, err := collection.UpdateOne(ctx, filter, update)
		if err != nil {
			log.Printf("Error updating document: %v", err)
			http.Error(w, "Error updating document", http.StatusInternalServerError)
			return
		}

		// Return success response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":   true,
			"modified":  result.ModifiedCount,
			"completed": req.Completed,
		})
	}
}

func setupRoutes(mongoClient *mongo.Client, config *Config) http.Handler {
	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("/api/documents", getDocumentsHandler(mongoClient, config))
	mux.HandleFunc("/api/documents/complete", markDocumentCompleteHandler(mongoClient, config))

	// Serve static frontend files
	uiPath, err := filepath.Abs("ui")
	if err != nil {
		log.Fatalf("Invalid UI path: %v", err)
	}

	// Create file server
	fileServer := http.FileServer(http.Dir(uiPath))

	// Route all other requests to the file server
	mux.Handle("/", fileServer)

	return mux
}

func main() {
	// Load configuration
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create MongoDB client
	mongoClient, err := createMongoClient(config)
	if err != nil {
		log.Fatalf("Failed to create MongoDB client: %v", err)
	}
	defer mongoClient.Disconnect(context.Background())

	// Check if sync flag is provided
	if len(os.Args) > 1 && os.Args[1] == "--sync" {
		// Create Jira client
		jiraClient, err := createJiraClientWithBackoff(config)
		if err != nil {
			log.Fatalf("Failed to create Jira client: %v", err)
		}

		// Sync issues
		if err := syncIssues(jiraClient, mongoClient, config); err != nil {
			log.Fatalf("Failed to sync issues: %v", err)
		}

		log.Println("Sync completed successfully")
		return
	}

	// Set up HTTP server
	router := setupRoutes(mongoClient, config)
	serverAddr := fmt.Sprintf(":%d", config.Server.Port)

	log.Printf("Starting server on %s", serverAddr)

	if err := http.ListenAndServe(serverAddr, router); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
