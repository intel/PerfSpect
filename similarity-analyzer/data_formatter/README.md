# data_formatter

Formats csv/xlsx data into json and helps comparing metrics from multiple csv's.

## Example usage

1. To compare TMA and basic metrics

`python3 main.py -f "summary_resnet.xlsx,summary_specjbb.xlsx" -m d `

2. To compare all compatible metrics

`python3 main.py -f "summary_resnet.xlsx,summary_specjbb.xlsx" -o comp1.csv`
