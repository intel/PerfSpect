# pmu-checker

Allows us to verify if the system is running any drivers/daemons that may be programming the PMU.

pmu-checker specifically checks if the following MSRs are actively being programmed/used :
1. 0x309
2. 0x30a
3. 0x30b 
4. 0xc1
5. 0xc2 
6. 0xc3 
7. 0xc4

## Usage
Usage: sudo ./pmu-checker [OPTION ...]


Options:

    -help, --help                   Show the current help and usage message, and exit
    -cpu, --cpu (int)               ReadMSRs from specific CPU, Default is 0
    -logfile, --logfile (string)    Specify the log filename to be used for logging, Default is "pmu-checker.log"
    -debug, --debug                 Set the loglevel to debug, Default is info
    -no-stdout, --no-stdout         Set the logwriter to write to log file only
