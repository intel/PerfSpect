#!/bin/bash

# Print CSV header
header=$(python3 /home/jharper5/dev/perfspect/scripts/repoactivity.py --header-only)
echo "Month,$header"

# Get the current date
current_date=$(date +%Y-%m-%d)

# Default values for owner and repo
owner=intel
repo=perfspect

# Override owner and repo with environment variables if provided
if [ -n "$GITHUB_OWNER" ]; then
  owner=$GITHUB_OWNER
fi

if [ -n "$GITHUB_REPO" ]; then
  repo=$GITHUB_REPO
fi

# Loop through the last 6 months
for i in {5..0}
do
  # Calculate the start date for the month
  start_date=$(date -d "$current_date -$i month -$(($(date +%d)-1)) days" +%Y-%m-01)
  
  # Calculate the end date for the month
  end_date=$(date -d "$start_date +1 month -1 day" +%Y-%m-%d)

    # Extract the month name
  month_name=$(date -d "$start_date" +%B)
  
  # Call the Python script with the calculated dates and append the output to the CSV
  result=$(python3 /home/jharper5/dev/perfspect/scripts/repoactivity.py --output csv "$owner" "$repo" "$start_date" "$end_date")

  # Print the month name and the result
  echo "$month_name,$result"
done