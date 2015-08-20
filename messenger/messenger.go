package main

import (
    "time"
    "net/url"
    "net/http"
    "strings"
    "github.com/garyburd/redigo/redis"
)

func getClient() redis.Conn {
    conn, err := redis.Dial("tcp", ":6379")
    if err != nil {
        panic("Unable to connect to redis: " + err.Error())
    }
    return conn
}

func getUrl() string {
    client := getClient()
    defer client.Close()
    url, err := redis.String(client.Do("GET", "url"))
    if err != nil {
        panic("Unable to load url to POST to: " + err.Error())
    }
    return url
}

func main() {
    client := getClient()

    day := strings.ToLower(time.Now().Weekday().String())

    usersRemoteEvery, _ := redis.Strings(client.Do("SMEMBERS", "remote.every." + day))
    usersRemoteOnce, _ := redis.Strings(client.Do("SMEMBERS", "remote.this." + day))

    usersRemote := append(usersRemoteEvery, usersRemoteOnce...)

    users := map[string]string{}
    for _, user := range usersRemote {
        users[user] = user
    }

    for user, _ := range users {
        client.Do("SREM", "remote.this." + day, user)
        if notRemote, _ := redis.Bool(client.Do("SISMEMBER", "not.remote.this." + day, user)); notRemote {
            client.Do("SREM", "not.remote.this." + day, user)
            delete(users, user)
        }
    }

    if len(users) <= 0 {
        return // nothing to do
    }

    remoteText := "Remote Today\n\n"
    for user, _ := range users {
        remoteText += "><@" + user + ">\n"
    }

    form := url.Values{}
    form.Add("payload", "{\"channel\": \"#team-status\", \"icon_emoji\": \":house:\", \"username\": \"Status Messenger\", \"text\": \"" + remoteText + "\"}")
    req, _ := http.NewRequest("POST", getUrl(), strings.NewReader(form.Encode()))
    req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
    poster := http.Client{}
    poster.Do(req)
}
