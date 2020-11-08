package main

import (
        "encoding/json"
        "errors"
        "fmt"
        "io/ioutil"
        "log"
        "net/http"
        "os"
        "strings"

        "golang.org/x/net/context"
        "golang.org/x/oauth2"
        "golang.org/x/oauth2/google"
        "google.golang.org/api/tasks/v1"
)

const (
        DefaultListName = "Default List"
        TasklistsMaxResults = 10
        TasksMaxResults = 100
)

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
        // The file token.json stores the user's access and refresh tokens, and is
        // created automatically when the authorization flow completes for the first
        // time.
        tokFile := "token.json"
        tok, err := tokenFromFile(tokFile)
        if err != nil {
                tok = getTokenFromWeb(config)
                saveToken(tokFile, tok)
        }
        return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
        authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
        fmt.Printf("Go to the following link in your browser then type the "+
                "authorization code: \n%v\n", authURL)

        var authCode string
        if _, err := fmt.Scan(&authCode); err != nil {
                log.Fatalf("Unable to read authorization code: %v", err)
        }

        tok, err := config.Exchange(context.TODO(), authCode)
        if err != nil {
                log.Fatalf("Unable to retrieve token from web: %v", err)
        }
        return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
        f, err := os.Open(file)
        if err != nil {
                return nil, err
        }
        defer f.Close()
        tok := &oauth2.Token{}
        err = json.NewDecoder(f).Decode(tok)
        return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
        fmt.Printf("Saving credential file to: %s\n", path)
        f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
        if err != nil {
                log.Fatalf("Unable to cache oauth token: %v", err)
        }
        defer f.Close()
        json.NewEncoder(f).Encode(token)
}

func listAllTasklists(srv *tasks.Service) error {
        r, err := srv.Tasklists.List().MaxResults(TasklistsMaxResults).Do()
        if err != nil {
                return err
        }
        fmt.Println("Task Lists:")
        if len(r.Items) > 0 {
                for _, i := range r.Items {
                        fmt.Printf("- %v\n", i.Title)
                }
        }
        return nil
}

func getIdFromName(srv *tasks.Service, name string) (string, error) {

        r, err := srv.Tasklists.List().MaxResults(TasklistsMaxResults).Do()
        if err != nil {
                return "", err
        }

        if len(r.Items) > 0 {
                for _, i := range r.Items {
                        //fmt.Printf("%s (%s)\n", i.Title, i.Id)
                        if i.Title == name {
                                return i.Id, nil
                        }
                }
        }

        return "", errors.New("no matching list")
}

func listTasksInTasklist(srv *tasks.Service, tasklistId string, tasklistName string) error {

        r, err := srv.Tasks.List(tasklistId).MaxResults(TasksMaxResults).Do()
        if err != nil {
                return err
        }

        fmt.Printf("Tasks in %v:\n", tasklistName)
        if len(r.Items) > 0 {
                for _, i := range r.Items {
                        fmt.Printf("- %s\n", i.Title)
                }
        }

        return nil
}

func moveTasksFromListToList(srv *tasks.Service, fromListId string, toListId string) error {
        r, err := srv.Tasks.List(fromListId).MaxResults(TasksMaxResults).Do()
        if err != nil {
                return err
        }

        if len(r.Items) > 0 {
                for _, i := range r.Items {
                        fmt.Printf("moving %s\n", i.Title)
                        _, err := srv.Tasks.Insert(toListId, i).Do()
                        if err != nil {
                                return err
                        }
                        err = srv.Tasks.Delete(fromListId, i.Id).Do()
                        if err != nil {
                                return err
                        }
                }
        }

        return nil
}

func main() {

        b, err := ioutil.ReadFile("credentials.json")
        if err != nil {
                log.Fatalf("Unable to read client secret file: %v", err)
        }

        // If modifying these scopes, delete your previously saved token.json.
        config, err := google.ConfigFromJSON(b, tasks.TasksReadonlyScope)
        if err != nil {
                log.Fatalf("Unable to parse client secret file to config: %v", err) }
        client := getClient(config)

        srv, err := tasks.New(client)
        if err != nil {
                log.Fatalf("Unable to retrieve tasks Client %v", err)
        }

        args := os.Args[1:]
        cmd := strings.Join(args, " ")

        if cmd == "list tasks" {
                // list tasks from default tasklist
                lname := DefaultListName
                id, err := getIdFromName(srv, lname)
                if err != nil {
                        log.Fatalf("Unable to find task list '%v': %v", lname, err)
                }
                listTasksInTasklist(srv, id, lname)
        } else if cmd == "list all tasks" {
                // list all from all task lists
        } else if cmd == "list tasklists" {
                err := listAllTasklists(srv)
                if err != nil {
                        log.Fatalf("Unable to list task lists: %v", err)
                }
        } else if len(args) > 3 && strings.HasPrefix(cmd, "list tasks in ") {
                // list tasks in "tasklist name"
                tasklistName := strings.Join(args[3:], " ")
                id, err := getIdFromName(srv, tasklistName)
                if err != nil {
                        log.Fatalf("Unable to find task list '%v': %v", tasklistName, err)
                }
                listTasksInTasklist(srv, id, tasklistName)
        } else if len(args) > 5 && strings.HasPrefix(cmd, "move tasks from") {
                // move tasks from "A list" to "Default list"
                var fromList, toList, buf []string
                for _, word := range args[3:] {
                        if word == "to" {
                                fromList = buf
                                buf = []string{}
                        } else {
                                buf = append(buf, word)
                        }
                }
                toList = buf
                fromListName := strings.Join(fromList, " ")
                toListName := strings.Join(toList, " ")
                fromListId, err := getIdFromName(srv, fromListName)
                if err != nil {
                        log.Fatalf("Unable to find task list '%v': %v", fromListId, err)
                }
                toListId, err := getIdFromName(srv, toListName)
                if err != nil {
                        log.Fatalf("Unable to find task list '%v': %v", toListName, err)
                }
                fmt.Printf("from: %v, to: %v\n", fromListId, toListId)

                err = moveTasksFromListToList(srv, fromListId, toListId)
                if err != nil {
                        log.Fatalf("Unable to move tasks from %v to %v: %v", fromListName, toListName, err)
                }
        }
}
