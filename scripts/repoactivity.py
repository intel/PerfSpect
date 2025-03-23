#!/usr/bin/env python3
# repoactivity.py
# Analyze GitHub repository activity
# Usage: python repoactivity.py owner repo start_date end_date [--output csv|human] [--header-only] [--token token]
# Example: python repoactivity.py octocat hello-world 2021-01-01 2021-12-31 --output csv --token YOUR_GITHUB_TOKEN
import requests
import argparse
import os
from datetime import datetime

# Parse command line arguments
parser = argparse.ArgumentParser(description='Analyze GitHub repository activity.')
parser.add_argument('owner', type=str, nargs='?', help='GitHub repository owner')
parser.add_argument('repo', type=str, nargs='?', help='GitHub repository name')
parser.add_argument('start_date', type=str, nargs='?', help='Start date in YYYY-MM-DD format')
parser.add_argument('end_date', type=str, nargs='?', help='End date in YYYY-MM-DD format')
parser.add_argument('--output', type=str, choices=['csv', 'human'], default='human', help='Output format: csv or human-readable')
parser.add_argument('--header-only', action='store_true', help='Print only the CSV header')
parser.add_argument('--token', type=str, help='GitHub access token')
args = parser.parse_args()

# Print the CSV header if the --header-only option is specified
if args.header_only:
    print("Start Date,End Date,Commits,Issues Opened,Issues Closed,Pull Requests Opened,Pull Requests Merged")
    exit(0)

# Ensure owner, repo, start_date, and end_date are provided if --header-only is not specified
if not args.owner or not args.repo or not args.start_date or not args.end_date:
    parser.error("the following arguments are required: owner, repo, start_date, end_date")

# Define the date range
start_date = f'{args.start_date}T00:00:00Z'
end_date = f'{args.end_date}T23:59:59Z'

# Define the headers for the API requests
headers = {
    'Accept': 'application/vnd.github.v3+json'
}
if args.token:
    headers['Authorization'] = f'token {args.token}'

# Get commits with pagination
commits_url = f'https://api.github.com/repos/{args.owner}/{args.repo}/commits'
commits_params = {'since': start_date, 'until': end_date, 'per_page': 100}
commits = []
while commits_url:
    commits_response = requests.get(commits_url, headers=headers, params=commits_params)
    commits_page = commits_response.json()
    commits.extend(commits_page)
    commits_url = commits_response.links.get('next', {}).get('url')

# Get issues with pagination and check dates
issues_url = f'https://api.github.com/repos/{args.owner}/{args.repo}/issues'
issues_params = {'since': start_date, 'state': 'all', 'direction': 'asc', 'per_page': 100}
issues = []
while issues_url:
    issues_response = requests.get(issues_url, headers=headers, params=issues_params)
    issues_page = issues_response.json()
    for issue in issues_page:
        if 'pull_request' in issue:
            continue
        created_at = datetime.strptime(issue['created_at'], '%Y-%m-%dT%H:%M:%SZ')
        if created_at > datetime.strptime(end_date, '%Y-%m-%dT%H:%M:%SZ'):
            break
        issues.append(issue)
    issues_url = issues_response.links.get('next', {}).get('url')

# Get pull requests with pagination and check dates
pulls_url = f'https://api.github.com/repos/{args.owner}/{args.repo}/pulls'
pulls_params = {'state': 'all', 'direction': 'desc', 'per_page': 100}
pulls = []
stop_retrieving = False
while pulls_url and not stop_retrieving:
    pulls_response = requests.get(pulls_url, headers=headers, params=pulls_params)
    pulls_page = pulls_response.json()
    for pull in pulls_page:
        created_at = datetime.strptime(pull['created_at'], '%Y-%m-%dT%H:%M:%SZ')
        if created_at < datetime.strptime(start_date, '%Y-%m-%dT%H:%M:%SZ'):
            stop_retrieving = True
            break
        pulls.append(pull)
    pulls_url = pulls_response.links.get('next', {}).get('url')

# Count the number of commits
num_commits = len(commits)

# Count the number of issues opened and closed in the date range
num_issues_opened = 0
num_issues_closed = 0
for issue in issues:
    created_at = datetime.strptime(issue['created_at'], '%Y-%m-%dT%H:%M:%SZ')
    if start_date <= issue['created_at'] <= end_date:
        num_issues_opened += 1
    if issue['closed_at']:
        closed_at = datetime.strptime(issue['closed_at'], '%Y-%m-%dT%H:%M:%SZ')
        if start_date <= issue['closed_at'] <= end_date:
            num_issues_closed += 1

# Count the number of pull requests opened and merged in the date range
num_pulls_opened = 0
num_pulls_merged = 0
for pull in pulls:
    created_at = datetime.strptime(pull['created_at'], '%Y-%m-%dT%H:%M:%SZ')
    if start_date <= pull['created_at'] <= end_date:
        num_pulls_opened += 1
    if pull['merged_at']:
        merged_at = datetime.strptime(pull['merged_at'], '%Y-%m-%dT%H:%M:%SZ')
        if start_date <= pull['merged_at'] <= end_date:
            num_pulls_merged += 1

# Print the results
if args.output == 'csv':
    print(f"{args.start_date},{args.end_date},{num_commits},{num_issues_opened},{num_issues_closed},{num_pulls_opened},{num_pulls_merged}")
else:
    print(f"Start Date: {args.start_date}")
    print(f"End Date: {args.end_date}")
    print(f"Number of commits: {num_commits}")
    print(f"Number of issues opened: {num_issues_opened}")
    print(f"Number of issues closed: {num_issues_closed}")
    print(f"Number of pull requests opened: {num_pulls_opened}")
    print(f"Number of pull requests merged: {num_pulls_merged}")