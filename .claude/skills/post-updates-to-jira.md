---
skill: post-updates-to-jira
description: Review recent commits and update active Jira tickets in ROSAENG-61383 epic
---

# Post Updates to Jira

This skill reviews git commits since the last Jira update and posts progress to active tickets in the ROSAENG-61383 epic.

## Steps

1. **Identify the last Jira update time**
   - Search Jira epic ROSAENG-61383 for the most recent comment
   - Use that timestamp as the baseline for "new work"

2. **Gather commits since last update**
   - Use `git log --since="<timestamp>" --pretty=format:"%h - %s (%ar)" --no-merges`
   - Group commits by topic/component
   - Identify which Jira tickets they relate to

3. **Analyze work completed**
   - Review commit messages and changes
   - Identify:
     - Features completed
     - Bugs fixed
     - Documentation updated
     - Test coverage changes
     - New files added
   - Categorize by relevant Jira ticket

4. **Check Jira ticket status**
   - Query epic ROSAENG-61383 for linked issues
   - Use `mcp__atlassian__searchJiraIssuesUsingJql` with JQL: `"epic link" = ROSAENG-61383`
   - Identify which tickets are still active (not Done/Closed)

5. **Post updates to active tickets**
   - For each active ticket with related commits:
     - Use `mcp__atlassian__addCommentToJiraIssue` to add progress comment
     - Include:
       - Summary of work completed
       - Relevant commit hashes
       - Updated metrics (test coverage, file counts, etc.)
       - Next steps if applicable
   - Format as markdown for readability

6. **Update epic with overall progress**
   - Post summary comment to ROSAENG-61383
   - Include:
     - Date range of commits
     - Number of commits
     - Summary of major changes
     - Updated completion status
     - Link to repository

## Example Update Format

For individual tickets:
```markdown
## Progress Update - <date>

### Work Completed
- Implemented X feature (commit abc123)
- Added Y documentation (commit def456)
- Fixed Z issue (commit ghi789)

### Metrics
- Test coverage: 73% (+3%)
- Files changed: 12
- Lines added: +450/-120

### Status
[Brief status summary]

### Next Steps
- [If applicable]
```

For epic:
```markdown
## Development Update - <date range>

### Summary
<number> commits pushed since <last update date>

### Major Changes
1. **Component 1**: [summary]
2. **Component 2**: [summary]
3. **Documentation**: [summary]

### Ticket Status
- ROSAENG-61389: ✅ Complete (marker-scanner)
- ROSAENG-61384: ✅ Complete (passthrough-gen)
- ROSAENG-61387: ✅ Complete (openapi-gen POC)

### Repository
https://github.com/openshift-online/hyperfleet-api-codegen

### Metrics
- Overall test coverage: 73%
- Total commits: <number>
- Files in repo: <number>
```

## Usage

```
/post-updates-to-jira
```

The skill will automatically:
- Find the last update timestamp
- Gather and analyze commits
- Update relevant Jira tickets
- Provide a summary of updates posted

## Requirements

- Atlassian MCP server must be configured
- Access to ROSAENG project in Jira
- Git repository must have recent commits
