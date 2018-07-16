import logging

logging.basicConfig(level=logging.INFO,
                    format='%(threadName)s %(asctime)s - %(name)s - %(levelname)s - %(message)s')
logging.getLogger('botocore').setLevel(logging.DEBUG)
logging.getLogger('boto3').setLevel(logging.DEBUG)

def getLogger(file):
    LOGGER = logging.getLogger(file)
    LOGGER.setLevel(logging.INFO)
    return LOGGER