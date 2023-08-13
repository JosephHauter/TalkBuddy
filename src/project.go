package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var client *mongo.Client
var postsCollection *mongo.Collection

type Post struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	Title     string             `bson:"title"`
	Content   string             `bson:"content"`
	CreatedAt time.Time          `bson:"created_at"`
}

type Comment struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	PostID    primitive.ObjectID `bson:"post_id"`
	Content   string             `bson:"content"`
	CreatedAt time.Time          `bson:"created_at"`
}

func init() {
	clientOptions := options.Client().ApplyURI("mongodb+srv://test:test123@cluster0.tkhvpyt.mongodb.net/")
	var err error
	client, err = mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		log.Fatal("Failed to connect to MongoDB:", err)
	}

	// Check if the connection was successful
	err = client.Ping(context.Background(), nil)
	if err != nil {
		log.Fatal("Failed to ping MongoDB:", err)
	}

	// Set up the MongoDB collections
	postsCollection = client.Database("mydb").Collection("posts")
}

func createPost(post *Post) error {
	_, err := postsCollection.InsertOne(context.Background(), post)
	return err
}

func createComment(comment *Comment) error {
	_, err := client.Database("mydb").Collection("comments").InsertOne(context.Background(), comment)
	return err
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		// Handle post creation
		title := r.FormValue("title")
		content := r.FormValue("content")
		if title != "" && content != "" {
			post := &Post{
				Title:     title,
				Content:   content,
				CreatedAt: time.Now(),
			}
			err := createPost(post)
			if err != nil {
				http.Error(w, "Failed to create post", http.StatusInternalServerError)
				return
			}
		}
	}

	// Fetch all posts from the database
	var posts []*Post
	cursor, err := postsCollection.Find(context.Background(), nil)
	if err != nil {
		log.Println("Failed to fetch posts:", err)
		http.Error(w, "Failed to fetch posts", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.Background())
	for cursor.Next(context.Background()) {
		var post Post
		err := cursor.Decode(&post)
		if err != nil {
			log.Println("Failed to decode posts:", err)
			http.Error(w, "Failed to decode posts", http.StatusInternalServerError)
			return
		}
		posts = append(posts, &post)
	}
	if err := cursor.Err(); err != nil {
		log.Println("Failed to iterate over posts:", err)
		http.Error(w, "Failed to iterate over posts", http.StatusInternalServerError)
		return
	}

	// Fetch all comments from the database
	commentsMap := make(map[string][]*Comment)
	cursor, err = client.Database("mydb").Collection("comments").Find(context.Background(), nil)
	if err != nil {
		log.Println("Failed to fetch comments:", err)
		http.Error(w, "Failed to fetch comments", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.Background())
	for cursor.Next(context.Background()) {
		var comment Comment
		err := cursor.Decode(&comment)
		if err != nil {
			log.Println("Failed to decode comments:", err)
			http.Error(w, "Failed to decode comments", http.StatusInternalServerError)
			return
		}
		commentsMap[comment.PostID.Hex()] = append(commentsMap[comment.PostID.Hex()], &comment)
	}
	if err := cursor.Err(); err != nil {
		log.Println("Failed to iterate over comments:", err)
		http.Error(w, "Failed to iterate over comments", http.StatusInternalServerError)
		return
	}

	data := struct {
		Posts    []*Post
		Comments map[string][]*Comment
	}{
		Posts:    posts,
		Comments: commentsMap,
	}

	renderTemplate(w, "home.html", data)
}

func commentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		postID := r.FormValue("postID")
		content := r.FormValue("content")
		if postID != "" && content != "" {
			postObjID, err := primitive.ObjectIDFromHex(postID)
			if err != nil {
				http.Error(w, "Invalid post ID", http.StatusBadRequest)
				return
			}
			comment := &Comment{
				PostID:    postObjID,
				Content:   content,
				CreatedAt: time.Now(),
			}
			err = createComment(comment)
			if err != nil {
				log.Println("Failed to add comment:", err)
				http.Error(w, "Failed to add comment", http.StatusInternalServerError)
				return
			}
		}
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func renderTemplate(w http.ResponseWriter, tmpl string, data interface{}) {
	tmpl = fmt.Sprintf("templates/%s", tmpl)
	t, err := template.ParseFiles(tmpl)
	if err != nil {
		log.Println("Error rendering template:", err)
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
		return
	}
	t.Execute(w, data)
}

func main() {
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/comment", commentHandler)

	log.Println("Starting server on port 8080")
	http.ListenAndServe(":8080", nil)
}
