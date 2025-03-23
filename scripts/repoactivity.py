#!/usr/bin/env python3
import requests
import argparse
import os
from datetime import datetime

# Replace with your repository details
owner = 'intel'
repo = 'perfspect'
# token = os.getenv('GITHUB_ACCESS_TOKEN')

# if not token:
#     raise ValueError("GITHUB_ACCESS_TOKEN environment variable is not set")

headers = {
    # 'Authorization': f'token {token}',
    'Accept': 'application/vnd.github.v3+json'
}

# Parse command line arguments
parser = argparse.ArgumentParser(description='Analyze GitHub repository activity.')
parser.add_argument('start_date', type=str, help='Start date in YYYY-MM-DD format')
parser.add_argument('end_date', type=str, help='End date in YYYY-MM-DD format')
args = parser.parse_args()

# Define the date range
start_date = f'{args.start_date}T00:00:00Z'
end_date = f'{args.end_date}T23:59:59Z'

# Get commits
commits_url = f'https://api.github.com/repos/{owner}/{repo}/commits'
commits_params = {'since': start_date, 'until': end_date}
commits_response = requests.get(commits_url, headers=headers, params=commits_params)
commits = commits_response.json()

# Get issues with pagination and check dates
issues_url = f'https://api.github.com/repos/{owner}/{repo}/issues'
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
pulls_url = f'https://api.github.com/repos/{owner}/{repo}/pulls'
pulls_params = {'state': 'all', 'direction': 'asc', 'per_page': 100}
pulls = []
while pulls_url:
    pulls_response = requests.get(pulls_url, headers=headers, params=pulls_params)
    pulls_page = pulls_response.json()
    for pull in pulls_page:
        created_at = datetime.strptime(pull['created_at'], '%Y-%m-%dT%H:%M:%SZ')
        if created_at > datetime.strptime(end_date, '%Y-%m-%dT%H:%M:%SZ'):
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

# Print the results in CSV format
print(f"{args.start_date},{args.end_date},{num_commits},{num_issues_opened},{num_issues_closed},{num_pulls_opened},{num_pulls_merged}")