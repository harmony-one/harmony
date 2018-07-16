import logging

logging.basicConfig(level=logging.INFO,
                    format='%(threadName)s %(asctime)s - %(name)s - %(levelname)s - %(message)s')

def getLogger(file):
    LOGGER = logging.getLogger(file)
    LOGGER.setLevel(logging.INFO)
    return LOGGER