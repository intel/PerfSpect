# pmu-checker

Allows us to verify if the system is running any drivers/daemons that may be programming the PMU. Superuseful for customers who have such instrumentations, don’t know it.

pmu-checker specifically checks if the following MSRs are actively being programmed/used:

1. 0x309
2. 0x30a
3. 0x30b
4. 0x30c (SPR)
5. 0xc1
6. 0xc2
7. 0xc3
8. 0xc4
9. 0xc5 (SPR)
10. 0xc6 (SPR)
11. 0xc7 (SPR)
12. 0xc8 (SPR)

## Usage

Usage: sudo ./pmu-checker [OPTION ...]

Options:

    -help, --help                   Show the current help and usage message, and exit
    -cpu, --cpu (int)               ReadMSRs from specific CPU, Default is 0
    -logfile, --logfile (string)    Specify the log filename to be used for logging, Default is "pmu-checker.log"
    -debug, --debug                 Set the loglevel to debug, Default is info
    -no-stdout, --no-stdout         Set the logwriter to write to log file only

## Contribution

If you are interested in contributing, feel free to fork this repo and create MR(Merge Requests).
