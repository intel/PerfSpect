import logging
import sys
import os


def crash(msg):
    logging.error(msg)
    sys.exit(1)


def configure_logging(output_dir):
    run_file = os.path.split(sys.argv[0])[1]
    program_name = os.path.splitext(run_file)[0]
    logfile = f"{os.path.join(output_dir, program_name)}.log"
    # create the log file if it doesn't already exist so that we can allow
    # writes from any user in case program is run under sudo and then later
    # without sudo
    if not os.path.exists(logfile):
        with open(logfile, "w+"):
            pass
        os.chmod(logfile, 0o666)  # nosec
    logging.basicConfig(
        level=logging.NOTSET,
        format="%(asctime)s %(levelname)s: %(message)s",
        handlers=[logging.FileHandler(logfile), logging.StreamHandler(sys.stdout)],
    )
    return logfile
