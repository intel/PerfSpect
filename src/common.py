import logging
import sys

log = logging.getLogger(__name__)


def crash(msg):
    log.error(msg)
    sys.exit(1)
