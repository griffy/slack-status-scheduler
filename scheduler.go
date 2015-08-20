package main

import (
    "net/http"
    "io"
    "strings"
    "github.com/garyburd/redigo/redis"
)

// /remote every tuesday thursday
// /remote this tuesday thursday
// /notremote every tuesday thursday
// /notremote this tuesday

func secretToken() string {
    client := getClient()
    defer client.Close()
    token, err := redis.String(client.Do("GET", "secret"))
    if err != nil {
        panic("Unable to load secret token: " + err.Error())
    }
    return token
}

func getClient() redis.Conn {
    conn, err := redis.Dial("tcp", ":6379")
    if err != nil {
        panic("Unable to connect to redis: " + err.Error())
    }
    return conn
}

func setRemoteEvery(username string, days []string) {
    client := getClient()
    defer client.Close()
    for day := range days {
        client.Do("SADD", "remote.every." + day, username)
    }
}

func setNotRemoteEvery(username string, days []string) {
    client := getClient()
    defer client.Close()
    for day := range days {
        client.Do("SREM", "remote.every." + day, username)
    }
}

func setRemoteOnce(username string, days []string) {
    client := getClient()
    defer client.Close()
    for day := range days {
        client.Do("SREM", "not.remote.this." + day, username)
        client.Do("SADD", "remote.this." + day, username)
    }
}

func setNotRemoteOnce(username string, days []string) {
    client := getClient()
    defer client.Close()
    for day := range days {
        client.Do("SREM", "remote.this." + day, username)
        client.Do("SADD", "not.remote.this." + day, username)
    }
}

func getSchedule(username string) string {
    schedule := ""

    days := []string{"sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"}

    everyWeek := []string{}
    thisWeek := []string{}
    notThisWeek := []string{}
    for day := range days {
        if remote, err := redis.Bool(client.Do("SISMEMBER", "remote.every." + day, username)); remote {
            everyWeek = append(everyWeek, day)
        }

        if remote, err := redis.Bool(client.Do("SISMEMBER", "remote.this." + day, username)); remote {
            thisWeek = append(thisWeek, day)
        }

        if notRemote, err := redis.Bool(client.Do("SISMEMBER", "not.remote.this." + day, username)); notRemote {
            notThisWeek = append(notThisWeek, day)
        }
    }

    if len(everyWeek) > 0 {
        schedule += "You are remote every " + strings.Join(everyWeek, ", ") + ".\n"
    }

    if len(thisWeek) > 0 {
        if len(everyWeek) > 0 {
            schedule += "In addition to your normal schedule, you are also remote this "
        } else {
            schedule += "You are remote this "
        }
        schedule += strings.Join(thisWeek, ", ") + ".\n"
    }

    if len(notThisWeek) > 0 {
        schedule += "You are not remote this " + strings.Join(notThisWeek, ", ") + ".\n"
    }

    return schedule
}

var secret string

func main() {
    secret = secretToken()

    http.HandleFunc("/scheduler/status", func(w http.ResponseWriter, r *http.Request) {
        token := r.FormValue("token")
        if token != secret {
            http.NotFound(w, r)
            return
        }

        name := r.FormValue("user_name")
        if name == "" {
            http.Error(w, "No username provided", 400)
            return
        }

        command := strings.ToLower(r.FormValue("command"))
        if command == "" {
            http.Error(w, "No command supplied", 400)
            return
        }

        if command != "schedule" {
            argString := strings.ToLower(r.FormValue("text"))
            if argString = "" {
                http.Error(w, "Invalid command", 400)
                return
            }

            args := strings.Split(argString, " ")

            type := args[0]
            days := args[1:]

            if command == "remote" && type == "this" {
                setRemoteOnce(name, days)
            } else if command == "remote" && type == "every" {
                setRemoteEvery(name, days)
            } else if command == "notremote" && type == "this" {
                setNotRemoteOnce(name, days)
            } else if command == "notremote" && type == "every" {
                setNotRemoteEvery(name, days)
            } else {
                http.Error(w, "Invalid command", 400)
                return
            }
        }

        // Output the schedule to the user
        w.Header().Set("Content-Type", "text/plain; charset=utf-8")
        w.WriteHeader(http.StatusOK)
        io.WriteString(w, getSchedule(username))
    })
}