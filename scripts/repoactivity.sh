#!/bin/bash
# This script generates a CSV file with the activity of a GitHub repository for the last 6 months
# Usage: ./repoactivity.sh
# The script will use the GITHUB_OWNER and GITHUB_REPO environment variables if provided otherwise
# it will use the default values: intel and perfspect, respectively.
# Example: GITHUB_OWNER=google GITHUB_REPO=protobuf ./repoactivity.sh

# Copyright (C) 2021-2025 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause


# Define the path to the repoactivity.py script
script_dir=$(dirname "$0")
repoactivity_py="$script_dir/repoactivity.py"

# Print CSV header
header=$(python3 "$repoactivity_py" --header-only)
echo "Month,$header"

# Get the current date
current_date=$(date +%Y-%m-%d)

# Default values for owner and repo, overridden by environment variables if set
owner=${GITHUB_OWNER:-intel}
repo=${GITHUB_REPO:-perfspect}

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
  result=$(python3 "$repoactivity_py" --output csv "$owner" "$repo" "$start_date" "$end_date")

  # Print the month name and the result
  echo "$month_name,$result"
done