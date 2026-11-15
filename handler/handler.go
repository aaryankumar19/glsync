package handler

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/ahmed-e-abdulaziz/glsync/code"
	"github.com/ahmed-e-abdulaziz/glsync/git"
)

type Handler struct {
	codeClient code.CodeClient
	git        git.GitClient
	startDate  *time.Time
}

func NewHandler(codeClient code.CodeClient, gitClient git.GitClient) Handler {
	return Handler{codeClient: codeClient, git: gitClient}
}

func NewHandlerWithStartDate(codeClient code.CodeClient, gitClient git.GitClient, startDate *time.Time) Handler {
	return Handler{codeClient: codeClient, git: gitClient, startDate: startDate}
}

// It does three things:
//
//	1- Fetch submissions using codeClient (skipping submissions on or before the last sync date)
//	2- Group submissions by UTC day and git commit each day's submissions in a single commit using max timestamp
//	3- Use git to push to the repo set in the git client
func (h Handler) Execute() {
	var cutoff *time.Time
	if h.startDate != nil {
		cutoff = h.startDate
	} else if h.git != nil {
		date, err := h.git.GetLastSyncDate()
		if err != nil {
			log.Printf("Warning: failed to get last sync date from git: %v\n", err)
		} else if date != nil {
			cutoff = date
		}
	}

	if cutoff != nil {
		log.Printf("Last sync date cutoff: %s. Submissions on or before this date will be skipped.\n", cutoff.UTC().Format("2006-01-02 15:04:05 UTC"))
	}

	var submissions []code.Submission
	var err error
	if cutoff != nil {
		submissions, err = h.codeClient.FetchSubmissionsSince(cutoff)
	} else {
		submissions, err = h.codeClient.FetchSubmissions()
	}
	if err != nil {
		panic("Error while fetching code submissions: " + err.Error())
	}

	if len(submissions) == 0 {
		log.Println("No new submissions to sync. Repository is up to date!")
		if h.git != nil {
			_ = h.git.Push()
		}
		return
	}

	log.Printf("Fetched %v submissions, will commit them next\n", len(submissions))

	// Group submissions by UTC day
	submissionsByDate := make(map[string][]code.Submission)
	for _, s := range submissions {
		if cutoff != nil && !s.LastSubmittedAt.After(*cutoff) {
			continue
		}
		dateKey := s.LastSubmittedAt.UTC().Format("2006-01-02")
		submissionsByDate[dateKey] = append(submissionsByDate[dateKey], s)
	}

	if len(submissionsByDate) == 0 {
		log.Println("No new submissions to sync after filtering. Repository is up to date!")
		if h.git != nil {
			_ = h.git.Push()
		}
		return
	}

	// Sort dates chronologically ascending (oldest to newest)
	dates := make([]string, 0, len(submissionsByDate))
	for date := range submissionsByDate {
		dates = append(dates, date)
	}
	sort.Strings(dates)

	for idx, dateKey := range dates {
		daySubs := submissionsByDate[dateKey]

		// Find the max submission time for this day
		maxTimestamp := daySubs[0].LastSubmittedAt.UTC()
		for _, s := range daySubs[1:] {
			t := s.LastSubmittedAt.UTC()
			if t.After(maxTimestamp) {
				maxTimestamp = t
			}
		}

		// Sort submissions within the day chronologically ascending
		sort.Slice(daySubs, func(i, j int) bool {
			return daySubs[i].LastSubmittedAt.Before(daySubs[j].LastSubmittedAt)
		})

		var files []git.CommitFile
		var bodyLines []string
		for _, s := range daySubs {
			fileName := h.buildFileName(s.Id, s.TitleSlug, s.Lang)
			folderName := fmt.Sprintf("%v %v", s.Id, s.Title)
			files = append(files, git.CommitFile{
				FolderName: folderName,
				FileName:   fileName,
				Code:       s.Code,
			})
			bodyLines = append(bodyLines, fmt.Sprintf("- %v %v", s.Id, s.Title))
		}

		questionWord := "questions"
		if len(daySubs) == 1 {
			questionWord = "question"
		}
		commitSubject := fmt.Sprintf("Code challenge submissions for %s (%d %s)", dateKey, len(daySubs), questionWord)
		commitMessage := commitSubject + "\n\n" + strings.Join(bodyLines, "\n")

		err := h.git.Commit(files, commitMessage, maxTimestamp)
		if err != nil && !strings.Contains(err.Error(), "nothing to commit") {
			log.Println("\t" + err.Error())
			log.Printf("\tEncountered an error while commiting the code for date: %v\n", dateKey)
		}
		log.Printf("\t%v%% days committed. Committed day no. %v of total %v (%s with %d submissions)\n",
			int(float64(idx+1)/float64(len(dates))*100), idx+1, len(dates), dateKey, len(daySubs))
	}
	err = h.git.Push()
	if err != nil {
		panic("Encountered an error while pushing to git, exiting...")
	}
}

// Takes the id, titleSlug and lang to return the fileName
//
// It will follow the format <id><titleSlug>.<langExtension>
//
// It figures out the lang extension using its internal langFileExtension map
// if there any code client support a new language then add it here to avoid future errors
// currently this map was only formed using LeetCode's lang name
func (Handler) buildFileName(id, titleSlug, lang string) string {
	langFileExtension := map[string]string{
		"cpp":        "cpp",
		"java":       "java",
		"python":     "py",
		"python3":    "py",
		"mysql":      "sql",
		"mssql":      "sql",
		"oraclesql":  "sql",
		"c":          "c",
		"csharp":     "cs",
		"javascript": "js",
		"typescript": "ts",
		"bash":       "sh",
		"php":        "php",
		"swift":      "swift",
		"kotlin":     "kt",
		"dart":       "dart",
		"golang":     "go",
		"ruby":       "rb",
		"scala":      "scala",
		"rust":       "rs",
		"racket":     "rkt",
		"erlang":     "erl",
		"elixir":     "ex",
		"postgresql": "sql",
	}
	return fmt.Sprintf("%s%s.%s", id, titleSlug, langFileExtension[lang])
}
