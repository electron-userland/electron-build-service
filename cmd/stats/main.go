package main

import (
  "fmt"
  "log"
  "os"
  "path/filepath"
  "regexp"
  "sort"
  "strings"
  "time"

  "database/sql"

  "github.com/develar/errors"
  "github.com/develar/go-fs-util"
  _ "github.com/go-sql-driver/mysql"
  "github.com/valyala/tsvreader"
)

const logDir = "/Volumes/test/logs"

func main() {
  err := do("build-service")
  if err != nil {
    log.Fatalf("%v", err)
  }
}

func do(databaseName string) error {
  files, err := getSortedFileNames()
  if err != nil {
    return err
  }

  db, err := sql.Open("mysql", "root@/logs")
  if err != nil {
    return err
  }

  defer db.Close()

  insertStatement, err := db.Prepare("INSERT INTO logs.builds VALUES( ?, ?, ?, ?)")
  if err != nil {
    return err
  }
  defer insertStatement.Close()

  tx, err := db.Begin()
  if err != nil {
    return err
  }

  err = process(db, files, insertStatement)
  if err != nil {
    tx.Rollback()
    return err
  }
  tx.Commit()
  return nil
}

func getSortedFileNames() ([]string, error) {
  files, err := fsutil.ReadDirContent(logDir)
  if err != nil {
    return nil, err
  }

  list := make([]string, 0)
  for _, file := range files {
    if strings.HasSuffix(file, ".tsv") {
      list = append(list, file)
    }
  }

  sort.Strings(list)

  return list, nil
}

func process(db *sql.DB, files []string, insertStatement *sql.Stmt) error {
  for _, file := range files {
    if !strings.HasSuffix(file, ".tsv") {
      continue
    }

    file, err := os.Open(filepath.Join(logDir, file))
    if err != nil {
      return err
    }

    err = processFile(file, insertStatement)
    if err != nil {
      return err
    }
  }

  return nil
}

func processFile(file *os.File, insertStatement *sql.Stmt) error {
  defer file.Close()

  oldBuildRe := regexp.MustCompile(`^Build \(([^)]+)\): (.+)`)
  oldTargetRe := regexp.MustCompile(`.+target=([a-zA-Z]+).+file=/stage/([^/]+)/.+`)

  var targets []string
  jobIdToTargets := make(map[string][]string)

  r := tsvreader.New(file)
  for r.Next() {
    // https://help.papertrailapp.com/kb/how-it-works/permanent-log-archives/

    // unique Papertrail event ID (64-bit integer as JSON string)
    r.SkipCol()

    // generated_at time
    generatedAtString := r.String()
    // received_at time
    r.SkipCol()

    // source_id
    r.SkipCol()
    // source_name
    r.SkipCol()
    // source_ip
    r.SkipCol()

    // facility_name
    r.SkipCol()
    // severity_name
    r.SkipCol()

    // program
    r.SkipCol()

    // message
    message := r.String()

    if strings.Contains(message, "• building") && !strings.Contains(message, "• building embedded") {
      t := oldTargetRe.FindStringSubmatch(message)
      jobId := t[2]
      jobIdToTargets[jobId] = append(jobIdToTargets[jobId], strings.ToLower(t[1]))
      continue
    }

    if strings.HasPrefix(message, "Building AppImage ") {
      targets = append(targets, "appimage")
      continue
    } else if strings.HasPrefix(message, "Building Snap ") {
      targets = append(targets, "snap")
      continue
    } else if strings.HasPrefix(message, "Building deb") {
      targets = append(targets, "snap")
      continue
    } else if !strings.HasPrefix(message, "Build (") {
      continue
    }

    results := oldBuildRe.FindStringSubmatch(message)
    jobId := results[1]

    targetsFromMap := jobIdToTargets[jobId]
    if targetsFromMap != nil && len(targetsFromMap) > 0 {
      targets = targetsFromMap
    }

    if targets == nil || len(targets) == 0 {
      if jobId == "j-vd1zSZ" || jobId == "k-aWMBbb" || jobId == "7e-sESlo9" || jobId == "c-lC0xlU" || jobId == "7-yXAI1r" {
        // job completed with error
        continue
      }

      return fmt.Errorf("no targets")
    }

    completedAt, err := time.Parse("2006-01-02T15:04:05", generatedAtString)
    if err != nil {
      return errors.WithStack(err)
    }

    duration, err := time.ParseDuration(strings.Replace(results[2], " ", "", -1))
    if err != nil {
      return errors.WithStack(err)
    }

    hours := int(duration.Hours())
    minutes := int(duration.Minutes()) % 60
    seconds := int(duration.Seconds()) % 60
    milliseconds := int(duration/time.Millisecond) - (seconds * 1000) - (minutes * 60000) - (hours * 3600000)

    insertStatement.Exec(jobId, fmt.Sprintf("%d:%d:%d.%d", hours, minutes, seconds, milliseconds), completedAt, strings.Join(targets, ","))
    if err != nil {
      return errors.WithStack(err)
    }

    targets = nil
  }

  return nil
}
