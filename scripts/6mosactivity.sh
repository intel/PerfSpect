#!/bin/bash

# Print CSV header
echo "Start Date,End Date,Commits,Issues Opened,Issues Closed,Pull Requests Opened,Pull Requests Merged"

# Get the current date
current_date=$(date +%Y-%m-%d)

# Loop through the last 6 months
for i in {0..5}
do
  # Calculate the start date for the month
  start_date=$(date -d "$current_date -$i month -$(($(date +%d)-1)) days" +%Y-%m-01)
  
  # Calculate the end date for the month
  end_date=$(date -d "$start_date +1 month -1 day" +%Y-%m-%d)
  
  # Call the Python script with the calculated dates and append the output to the CSV
  python3 /home/jharper5/dev/perfspect/scripts/repoactivity.py $start_date $end_date
done