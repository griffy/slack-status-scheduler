package main

import (
    "net/http"
    "io"
    "unicode"
    "strings"
    "github.com/garyburd/redigo/redis"
)

// /remote schedule
// /remote clear
// /remote every tuesday thursday
// /remote this tuesday thursday
// /notremote every tuesday thursday
// /notremote this tuesday

func getClient() redis.Conn {
    conn, err := redis.Dial("tcp", ":6379")
    if err != nil {
        panic("Unable to connect to redis: " + err.Error())
    }
    return conn
}

func secretTokens() map[string]string {
    client := getClient()
    defer client.Close()
    tokenList, err := redis.Strings(client.Do("SMEMBERS", "secrets"))
    if err != nil {
        panic("Unable to load secret tokens: " + err.Error())
    }
    tokens := map[string]string{}
    for _, token := range tokenList {
        tokens[token] = token
    }
    return tokens
}

func setRemoteEvery(username string, days []string) {
    client := getClient()
    defer client.Close()
    for _, day := range days {
        client.Do("SREM", "remote.this." + day, username)
        client.Do("SADD", "remote.every." + day, username)
    }
}

func setNotRemoteEvery(username string, days []string) {
    client := getClient()
    defer client.Close()
    for _, day := range days {
        client.Do("SREM", "not.remote.this." + day, username)
        client.Do("SREM", "remote.every." + day, username)
    }
}

func setRemoteOnce(username string, days []string) {
    client := getClient()
    defer client.Close()
    for _, day := range days {
        client.Do("SREM", "not.remote.this." + day, username)
        if remote, err := redis.Bool(client.Do("SISMEMBER", "remote.every." + day, username)); !remote && err == nil {
            client.Do("SADD", "remote.this." + day, username)
        }
    }
}

func setNotRemoteOnce(username string, days []string) {
    client := getClient()
    defer client.Close()
    for _, day := range days {
        client.Do("SREM", "remote.this." + day, username)
        if remote, err := redis.Bool(client.Do("SISMEMBER", "remote.every." + day, username)); remote && err == nil {
            client.Do("SADD", "not.remote.this." + day, username)
        }
    }
}

func clearAll(username string) {
    client := getClient()
    defer client.Close()

    days := []string{"sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"}

    for _, day := range days {
        client.Do("SREM", "remote.this." + day, username)
        client.Do("SREM", "not.remote.this." + day, username)
        client.Do("SREM", "remote.every." + day, username)
    }
}

func getSchedule(username string) string {
    schedule := ""

    client := getClient()
    defer client.Close()

    days := []string{"sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"}

    everyWeek := []string{}
    thisWeek := []string{}
    notThisWeek := []string{}
    for _, day := range days {
        readableDay := []rune(day)
        readableDay[0] = unicode.ToUpper(readableDay[0])

        if remote, _ := redis.Bool(client.Do("SISMEMBER", "remote.every." + day, username)); remote {
            everyWeek = append(everyWeek, string(readableDay))
        }

        if remote, _ := redis.Bool(client.Do("SISMEMBER", "remote.this." + day, username)); remote {
            thisWeek = append(thisWeek, string(readableDay))
        }

        if notRemote, _ := redis.Bool(client.Do("SISMEMBER", "not.remote.this." + day, username)); notRemote {
            notThisWeek = append(notThisWeek, string(readableDay))
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

    if len(everyWeek) == 0 && len(thisWeek) == 0 {
        schedule += "You have no remote days scheduled.\n"
    }

    return schedule
}

func filterDays(givenDays []string) []string {
    days := []string{}

    for _, day := range givenDays {
        switch day {
        case "sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday":
            days = append(days, day)
        }
    }

    return days
}

var secrets map[string]string

func main() {
    secrets = secretTokens()

    http.HandleFunc("/scheduler/status", func(w http.ResponseWriter, r *http.Request) {
        token := r.FormValue("token")
        if _, ok := secrets[token]; !ok {
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

        argString := strings.ToLower(r.FormValue("text"))
        if argString == "" {
            http.Error(w, "Invalid command", 400)
            return
        }

        args := strings.Split(argString, " ")

        subcommand := args[0]

        if command == "/remote" && subcommand == "this" && len(args) > 1 {
            days := filterDays(args[1:])
            setRemoteOnce(name, days)
        } else if command == "/remote" && subcommand == "every" && len(args) > 1 {
            days := filterDays(args[1:])
            setRemoteEvery(name, days)
        } else if command == "/notremote" && subcommand == "this" && len(args) > 1 {
            days := filterDays(args[1:])
            setNotRemoteOnce(name, days)
        } else if command == "/notremote" && subcommand == "every" && len(args) > 1 {
            days := filterDays(args[1:])
            setNotRemoteEvery(name, days)
        } else if command == "/remote" && subcommand == "clear" {
            clearAll(name)
        } else if command == "/remote" && subcommand == "schedule" {
            // noop
        } else {
            http.Error(w, "Invalid command", 400)
            return
        }

        // Output the schedule to the user
        w.Header().Set("Content-Type", "text/plain; charset=utf-8")
        w.WriteHeader(http.StatusOK)
        io.WriteString(w, getSchedule(name))
    })

    http.ListenAndServe(":9999", nil)
}