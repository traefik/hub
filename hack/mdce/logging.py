#!/usr/bin/python3
"""
@brief  Module used to log messages to screen in a standard format with colour.
        This class is to be used statically, and provides some default values,
        with the ability to extend as required.

        For examples, run the script directly.

@author Kris Dunning (ippie52@gmail.com)
"""


class Log:
    """
    Logging class used to display log messages
    """
    DEFAULT_PREFIX_ERROR = 'E: '
    DEFAULT_PREFIX_WARNING = 'W: '
    DEFAULT_PREFIX_INFO = 'I: '
    DEFAULT_PREFIX_DEBUG = 'D: '

    TYPE_ERROR = 'error'
    TYPE_WARNING = 'warning'
    TYPE_INFO = 'info'
    TYPE_DEBUG = 'debug'
    TYPE_PREFIX = {
        TYPE_ERROR: DEFAULT_PREFIX_ERROR,
        TYPE_WARNING: DEFAULT_PREFIX_WARNING,
        TYPE_INFO: DEFAULT_PREFIX_INFO,
        TYPE_DEBUG: DEFAULT_PREFIX_DEBUG,
    }

    NORM = '\033[0m'
    COL_BLK = '\033[1;30m'
    COL_RED = '\033[1;31m'
    COL_GRN = '\033[1;32m'
    COL_BLU = '\033[1;34m'
    COL_MAG = '\033[1;35m'
    COL_YLW = '\033[1;33m'
    COL_CYN = '\033[1;36m'
    COL_WHT = '\033[1;37m'
    COL = {
        TYPE_ERROR: COL_RED,
        TYPE_WARNING: COL_YLW,
        TYPE_INFO: COL_GRN,
        TYPE_DEBUG: COL_BLU
    }

    VERB_OFF = 0
    VERB_ERROR = 1
    VERB_WARNING = 2
    VERB_INFO = 3
    VERB_DEBUG = 4
    VERB = {
        TYPE_ERROR: VERB_ERROR,
        TYPE_WARNING: VERB_WARNING,
        TYPE_INFO: VERB_INFO,
        TYPE_DEBUG: VERB_DEBUG,
    }
    CURRENT_VERBOSITY = VERB_DEBUG

    @staticmethod
    def set_verb(verb):
        """Sets the current verbosity level"""
        Log.CURRENT_VERBOSITY = verb


    @staticmethod
    def e(message, end='\n'):
        """Prints an error message"""
        Log.message(Log.TYPE_ERROR, message, end=end)

    @staticmethod
    def w(message, end='\n'):
        """Prints a warning message"""
        Log.message(Log.TYPE_WARNING, message, end=end)

    @staticmethod
    def i(message, end='\n'):
        """Prints an information message"""
        Log.message(Log.TYPE_INFO, message, end=end)

    @staticmethod
    def d(message, end='\n'):
        """Prints a debug message"""
        Log.message(Log.TYPE_DEBUG, message, end=end)

    @staticmethod
    def message(log_type, message, end='\n'):
        """
        Prints a message to the screen in a standard format for the given type
        """
        verb = Log.VERB[log_type]
        if verb <= Log.CURRENT_VERBOSITY:
            col = Log.COL[log_type]
            prefix = Log.TYPE_PREFIX[log_type]
            print(f"{col}{prefix}{message}{Log.NORM}", end=end)

    @staticmethod
    def set_error(colour=None, prefix=None):
        Log.set_log(Log.TYPE_ERROR, colour, prefix)

    @staticmethod
    def set_warning(colour=None, prefix=None):
        Log.set_log(Log.TYPE_WARNING, colour, prefix)

    @staticmethod
    def set_info(colour=None, prefix=None):
        Log.set_log(Log.TYPE_INFO, colour, prefix)

    @staticmethod
    def set_debug(colour=None, prefix=None):
        Log.set_log(Log.TYPE_DEBUG, colour, prefix)

    @staticmethod
    def set_log(log_type, colour=None, prefix=None, verb=None):
        """
        Sets the attributes of a log type, or adds a new one
        """
        if colour is not None:
            Log.COL[log_type] = colour
        elif log_type not in Log.COL:
            Log.COL[log_type] = Log.COL_WHT

        if prefix is not None:
            Log.TYPE_PREFIX[log_type] = prefix
        elif log_type not in Log.TYPE_PREFIX:
            Log.TYPE_PREFIX[log_type] = log_type

        if verb is not None:
            Log.VERB[log_type] = verb
        elif log_type not in Log.VERB:
            Log.VERB[log_type] = Log.VERB_ERROR

if __name__ == '__main__':

    # Print different default values
    Log.i('This is a basic info message, as per the defaults.')
    Log.e('This is a default error message.')
    Log.w('And this is a default warning message with a tab as the line ending', end='\t')
    Log.d('This is a default debug message')
    Log.set_info(colour=Log.COL_CYN)
    Log.i('Info messages are now in CYAN')
    Log.set_error(prefix='ERROR!!')
    Log.e('Error messages have had their prefix updated')
    Log.set_log('new_log', colour=Log.COL_MAG, prefix='New Message')
    Log.message('new_log', 'A new message type created called "new_log" in magenta.')
