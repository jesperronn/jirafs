package main

import (
	"fmt"
	"github.com/jirafs/jirafs/internal/board"
	"github.com/jirafs/jirafs/internal/schema"
)

func main() {
	// Recreate the exact test scenario to understand what's happening
	b := board.NewBoard()
	
	// Create test issues with different statuses exactly as in the test
	issues := []*schema.Issue{
		{
			Identity: schema.IssueIdentity{
				Key:     "PROJ-1",
				Type:    "story",
				Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
			},
			Status: "Open",
		},
		{
			Identity: schema.IssueIdentity{
				Key:     "PROJ-2",
				Type:    "bug",
				Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
			},
			Status: "In Progress",
		},
		{
			Identity: schema.IssueIdentity{
				Key:     "PROJ-3",
				Type:    "task",
				Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
			},
			Status: "Resolved",
		},
		{
			Identity: schema.IssueIdentity{
				Key:     "PROJ-4",
				Type:    "epic",
				Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
			},
			Status: "Custom Status",
		},
		{
			Identity: schema.IssueIdentity{
				Key:     "PROJ-5",
				Type:    "story",
				Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
			},
			Status: "",
		},
	}
	
	fmt.Println("Testing with these issues:")
	for i, issue := range issues {
		fmt.Printf("Issue %d: Key=%s, Status='%s'\n", i+1, issue.Identity.Key, issue.Status)
	}
	
	// Group issues by status - using nil registry for now to make tests pass
	b.GroupByStatus(issues, nil)
	
	fmt.Println("\nColumn order:", b.ColumnOrder)
	fmt.Println("Status columns:")
	for colName, issuesInCol := range b.StatusColumns {
		fmt.Printf("  %s: %d issues\n", colName, len(issuesInCol))
		for _, issue := range issuesInCol {
			fmt.Printf("    - %s\n", issue.Identity.Key)
		}
	}
	
	// Let's understand the test expectations better:
	fmt.Println("\nExpected:")
	fmt.Println("- 1 issue in 'Open' column")
	fmt.Println("- 1 issue in 'In Progress' column") 
	fmt.Println("- 1 issue in 'Resolved' column")
	fmt.Println("- 2 issues in 'Unknown' column (Custom Status and \"\")")
}