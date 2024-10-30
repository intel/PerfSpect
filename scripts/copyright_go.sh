#!/bin/bash

# Copyright (C) 2021-2024 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause

# Define the copyright header
COPYRIGHT_HEADER="// Copyright (C) 2021-2024 Intel Corporation\n// SPDX-License-Identifier: BSD-3-Clause"

# Loop through all .go files in the current directory and its subdirectories
find . -name "*.go" | while read -r file; do
    # Check if the file already contains the copyright header
    if ! grep -q "Copyright (C) 2021-2024 Intel Corporation" "$file"; then
        # Read the file line by line
        {
            while IFS= read -r line; do
                echo "$line"
                if [[ $line == package* ]]; then
                    echo -e "$COPYRIGHT_HEADER"
                fi
            done
        } < "$file" > temp_file && mv temp_file "$file"
        echo "Added copyright header to $file"
    else
        echo "Copyright header already present in $file"
    fi
done
