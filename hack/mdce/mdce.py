#!/usr/bin/python3
"""
@brief  Script used to embed code from files into markdown files

@author Kris Dunning (ippie52@gmail.com)
"""

from os import chdir, getcwd, walk, listdir, remove, chdir
from os.path import dirname, realpath, exists, isdir, isfile, join
from argparse import ArgumentParser
from re import search, IGNORECASE
from shutil import copyfile
from filecmp import cmp
from subprocess import Popen, PIPE, STDOUT, TimeoutExpired
from sys import exit
from logging import Log
from tempfile import TemporaryDirectory
import shlex
import pathlib

#
# Set up argument parsing for MDCE and command line arguments provided
#
parser = ArgumentParser(prog="Markdown Code Embed",
                        description="Embed code within markdown documents")
parser.add_argument('-d', '--directories', metavar="directory",
                    help='Directories to be scanned for *.md files',
                    default=[], nargs='+')
parser.add_argument('-f', '--files', metavar="file_name",
                    help='Files to be scanned',
                    default=[], nargs='+')
parser.add_argument('-e', '--exclude', metavar='directory', nargs='+',
                    help='Directories to exclude from searching',
                    default=[])
parser.add_argument('-i', '--include-self', action='store_true', default=False,
                    help="""
Includes the directory (and sub-directories with -s) of this script when parsing
""")
parser.add_argument('-s', '--sub', action="store_true",
                    help='Checks all sub-directories',
                    default=False)
parser.add_argument('-b', '--backup', action="store_true",
                    help='Backs up the original file, appending ".old" to the file name',
                    default=False)
parser.add_argument('-g', '--ignore-git', action="store_true",
                    help='Exit value ignores changes in git',
                    default=False)
parser.add_argument('-u', '--ignore-untracked', action="store_true",
                    help='Exit value ignores changes to untracked files',
                    default=False)
parser.add_argument('-q', '--quiet', action='store_true',
                    help='Reduces the number of messages printed',
                    default=False)
parser.add_argument('-c', '--code', action='store_true',
                    help='Displays the code parser usage instructions, then exit',
                    default=False)
parser.add_argument('-t', '--timeout', type=int, default=None, metavar='timeout',
                    help="""
The number of seconds to wait for the git requests to complete.
Default is no timeout.
""")
parser.add_argument('-C', '--commit', default="", metavar='commit message',
                    nargs='?', help="""
Commits any files changed during the run, either automatically if a commit message
is provided, or pauses allowing the user to create a commit message for the changes.
Please treat this option with caution!
""")

#
# Set up parser for block commands
#
code_parser = ArgumentParser(prog='Markdown Code Embed - Code Block Commands',
                             description='Code block command parsing')
code_parser.add_argument('-s', '--start', metavar='line_number', default=None,
                         help='The starting line to display from.')
code_parser.add_argument('-e', '--end', metavar='line_number', default=None,
                         help="""
The end line (inclusive) to display to. If start is set, this will automatically
be set to the same, so that only one line is shown. Leave both blank to include
the entire file.
""")
code_parser.add_argument('-i', '--indent', metavar='indent_option (t | sN)',
                         default=None,
                         help="""
Indentation options for re-indenting to trim leading white space.
Use 't' for code using tab characters for indentation.
Use 'sN' for code using spaces, where 'N' is replaced by the number of spaces
  per indentation, e.g. s4.
If this value is not set, any leading white space will be left in the block.
""")
code_parser.add_argument('-r', '--run', action='store_true', default=False,
                         help='Whether to run the file provided and capture the output from stdout.')
code_parser.add_argument('-a', '--args', default="",
                         help="""
Arguments passed forward when the run option is provided.
Single arguments must be wrapped in two sets of quotation marks, for example:
"'-h'" to avoid them being interpreted as another option.
Multiple arguments must be surrounded with quotation marks, however not all need
to be wrapped individually unless they need to preserve spaces, e.g.
'a1, "arg 2", three'.
""")
code_parser.add_argument('-t', '--timeout', type=int, default=None,
                         help="""
The number of seconds to wait for the process to complete.
Default is no timeout.
""")

#
# Helper functions
#
def getStrippedLines(lines, indent):
    """
    @brief  Strips the indentation from each lines to the minimum, where
            required
    @param  lines   The lines to be stripped
    @param  indent  The indent string
    @return Stripped lines
    """
    numbers = []
    for line in lines.copy():
        i = 0
        while line.startswith(indent):
            line = line[len(indent):]
            i += 1
        numbers.append(i)
    strip_count = min(numbers) * len(indent)
    return [line[strip_count:] for line in lines]


def getSourceLines(filename, start, end, indent=None):
    """
    @brief  Gets the list of lines to be extracted from the given file
    @param  filename    The name of the file to read from
    @param  start       The line number to start from
    @param  end         The line number to end on
    @param  indent      The indent string
    @return Gets the source lines from the given file, between start and end and
            with leading white space removed, where requested
    """
    selected = None
    with open(filename) as file:
        lines = file.readlines()
        # Bounds check
        if start is None:
            start = 1
            end = len(lines)
        elif end is None:
            end = start
        # Add one, zero indexed list vs lines starting at 1
        start = int(start) - 1
        # No need to remove from end
        end = int(end)
        if start <= len(lines) and end <= len(lines):
            Log.d(f"Grabbing lines {start} to {end}")
            selected = lines[start:end]
        else:
            raise IndexError(f"Line indices out of bounds: {start} {end} out of {len(lines)}" )

        if indent is not None:
            selected = getStrippedLines(selected, indent)

    return selected

def getRunnableLines(filename, args, timeout):
    """
    @brief  Gets the stdout from the given application or script with the given
            arguments
    @param  filename    The name of the file to run
    @param  args        The arguments to pass to the command
    @param  timeout     The process timeout duration in seconds
    @return The output from the command
    """
    args = [filename] + args if len(args) > 0 else filename

    Log.i(f'Running {args}')
    p = Popen(args, stdout=PIPE, stderr=PIPE)
    o = ""
    try:
        o, e = p.communicate(timeout=timeout)
        if p.returncode != 0:
            e = e.decode('utf-8')
            raise RuntimeError(f'Process failed: {args}: \n{e}')
        o = o.decode('utf-8')
    except TimeoutExpired as e:
        Log.e('Process timed out.')
        raise e
    return [o + '\n' for o in o.splitlines()]

#
# Helper class
#
class BlockInfo:
    """Simple class used to represent a code block start or end"""

    def __init__(self, is_start=False, is_end=False, length=0, filename=None,
                 # runnable=False, start_line=None, end_line=None, indent=None,
                 args=None):
        """
        @brief  Initialises the object
        @param  is_start    Whether this is a start block
        @param  is_end      Whether this is an end block
        @param  length      The number of dashes for this block
        @param  filename    The name of the file to be processed
        @param  args        Code block arguments
        """
        self._is_start = is_start
        self._is_end = is_end
        self._filename = filename
        self._length = length

        if args is None:
            args = ""

        block_args = code_parser.parse_args(shlex.split(args)) if args is not None else None
        self._runnable = block_args.run
        self._start_line = block_args.start
        self._end_line = block_args.end
        self._timeout = block_args.timeout

        if block_args.indent is not None:
            self._indent = BlockInfo.getTab(block_args.indent)
        else:
            self._indent = None
        self._args = shlex.split(block_args.args)

    @staticmethod
    def getTab(in_val):
        """
        @brief  Gets the tab character from the value input from the user
        @param  in_val  The input value to be parsed
        @return The string representation of the tab used in the code block
        """
        tab = '\t'
        if in_val.lower().startswith('s'): # Using Spaces
            # Default to 4 if no value is provided
            space_count = int(in_val[1:]) if len(in_val) > 1 else 4
            tab = ' ' * space_count
        elif in_val.lower() != 't':
            raise ValueError('Indentation must be either "sN" (N being an integer) or "t"')

        return tab

    def __repr__(self):
        """
        @brief  Gets the string representation of this object
        """
        if self._is_start:
            if self._runnable:
                return f'Start Block: Run {self._filename} <{self._args}>'
            else:
                return f'Start Block: File {self._filename} [{self._start_line}-{self._end_line}]'
        elif self._is_end:
            return 'End of code block'
        else:
            return 'No snippet info'

    def getRunnableArgs(self):
        """
        @brief  Gets the arguments to be passed to Popen.
        """
        args = []
        Log.d(f'Parsing {self._args}')
        if self._args is not None:
            if type(self._args) is str:
                try:
                    self._args = shlex.split(self._args)
                except Exception as e:
                    raise ValueError(f'Invalid arguments found: {self._args} - ' + str(e))

            elif type(self._args) is dict:
                raise ValueError(f'Dictionary objects are not supported: {self._args}')

            if type(self._args) is list:
                args = self._args
                Log.d(f'Args are list: {self._args}')
            elif type(self._args) is str:
                Log.d(f'Args are string: {self._args}')
                args = [self._args]
        return args

#
# More helper functions
#
def getBlockInfo(line, start_block):
    """
    @brief  Uses the current line to create a BlockInfo object
    @param  line        The current line of text from the file
    @param  start_block The last start block found, otherwise None
    @return The BlockInfo object for the current line
    """
    # Expression to extract the dashes, syntax, file name and any options to be
    # processed for this block
    expr = r"^\s*(?P<dash>```+)\s*((?P<syntax>\w+) :)?\s*(?P<filename>[\w_\-\.\/]+)?(?P<args>.*)$"
    block = search(expr, line, IGNORECASE)
    info = BlockInfo()

    if block is not None:
        if start_block is None:
            if block.group('syntax') is not None:
                # This is a start block to be processed further
                dash = block.group('dash')
                info = BlockInfo(is_start=True, length=len(dash),
                                 filename=block.group('filename'),
                                 args=block.group('args'))
            else:
                # This is a simple start block
                info = BlockInfo(is_start=True, length=len(block.group('dash')))

        elif start_block is not None and len(block.group('dash')) >= start_block._length:
            info = BlockInfo(is_end=True)

    return info


class LineParser(object):
    """
    @brief  Class used to parse a markdown file line by line
    """

    def __init__(self):
        """
        @brief  Initialises the object
        """
        self._last_block = None
        self._deferred_lines = None

    def parseLine(self, directory, line, defer=True):
        """
        @brief  Parses an incoming line, returns a tuple indicating whether to
                replace the original line, and what to replace it with.
        @param  directory   The directory of the file being parsed
        @param  line        The line to be parsed
        @param  defer   Whether to defer the output from a start block until the
                        end block is found. This helps prevent overwriting data
                        when no end block is found. Default True.
        @return tuple:
                (replace, lines)
                replace     Whether to replace the existing line
                lines       The lines to replace with (can be None)
        """
        replace = True
        block_info = getBlockInfo(line, self._last_block)
        out_lines = []
        if block_info._is_start:
            self._last_block = block_info
            out_lines.append(line)
            if block_info._filename is not None:
                fname = join(directory, block_info._filename)
                if block_info._runnable:
                    stdout_lines = getRunnableLines(fname,
                                                    block_info.getRunnableArgs(),
                                                    block_info._timeout)
                    self._deferred_lines = stdout_lines
                else:
                    source_lines = getSourceLines(fname,
                                                  block_info._start_line,
                                                  block_info._end_line,
                                                  block_info._indent)
                    self._deferred_lines = source_lines

            if defer or self._deferred_lines is None:
                replace = False
            else:
                # Append generated data to start block if not deferring
                out_lines += self._deferred_lines
                self._deferred_lines = None

        elif block_info._is_end:
            self._last_block = None
            # Add generated data before end block if available
            if defer and self._deferred_lines is not None:
                out_lines += self._deferred_lines
                self._deferred_lines = None
            else:
                replace = False

            out_lines.append(line)

        elif self._last_block is None or self._last_block._filename is None:
            out_lines.append(line)
            replace = False
        # No other action required, ignore these lines
        return (replace, out_lines)

class Parser(object):
    """
    Provides the base Parser class, on top of which, specific types of parsing
    can be used
    """

    def __init__(self):
        pass

    def _processLines(self, lines_tuple):
        """
        Processes the lines to write to the output
        @param lines_tuple  Tuple output from LineParser#parseLine
        """
        raise NotImplementedError("This must be implemented in the child class")

    def _parse(self, in_lines, directory, filename):
        """
        Parses the collection of lines
        @param  in_lines    The collection of lines to parse
        @param  directory   The directory containing the source
        @param  filename    The source file name
        """
        line_parser = LineParser()
        try:
            for num, line in enumerate(in_lines):
                if isinstance(line, bytes):
                    line = line.decode('utf-8')
                self._processLines(line_parser.parseLine(directory, line))
        except (IndexError, ValueError, RuntimeError, FileNotFoundError) as e:
            # Report the error with line number to make it easier to find
            Log.w(f'Failed to parse input [{num + 1}]: {filename}\n{e}')
            return False
        return True

class FileToFileParser(Parser):
    """
    Class to provide parsing for a markdown file with file input and output
    """

    def __init__(self):
        """Constructor"""
        super(FileToFileParser, self).__init__()
        self.temp_dir = None
        self.temp_file = None
        self.temp_file_name = None

    def _processLines(self, lines_tuple):
        """
        Process the lines to write to the output file
        @param lines_tuple  Tuple output from LineParser#parseLine
        """
        if self.temp_file is not None:
            self.temp_file.writelines(lines_tuple[1])
        else:
            raise RuntimeError('Attempting to process without an open file')

    def parse(self, filename, backup):
        """
        Parses the collection of lines and writes to an output file
        @param  filename    The name of the input file to parse
        @param  backup      Whether to back up the file before replacing
        @return True if the file has been updated, otherwise False if unchanged
        """
        parse_result = False
        self.temp_dir = TemporaryDirectory();
        self.temp_file_name = join(self.temp_dir.name, 'mdce_working.md')
        self.temp_file = open(self.temp_file_name, 'w')

        with open(filename, 'rb') as file:
            parse_result = super()._parse(file, dirname(filename), filename)
        self.temp_file.close()

        # Asses whether we need to back up and replace the existing file
        file_changed = parse_result and not cmp(self.temp_file_name, filename)
        if file_changed:
            if backup:
                old_file_name = filename + ".old"
                copyfile(filename, old_file_name)
            copyfile(self.temp_file_name, filename)

        # Remove temporary directory and its contents
        self.temp_dir.cleanup()

        return file_changed


def getFiles(root, check_subs, ignored_dirs):
    """
    @brief  Gets the matching files recursively (where required)
    @param  root            The root directory
    @param  check_subs      Whether to check sub-directories
    @param  ignored_dirs    Collection of directories to be ignored
    @return Collection of files to be processed
    """
    root = realpath(root)
    files = []
    # Must not start with any ignored directory
    if not root.startswith(tuple(ignored_dirs)):
        for path in listdir(root):
            path=realpath(join(root, path))
            if isfile(path) and pathlib.Path(path).suffix == '.md':
                Log.i(f'Found file: {path}')
                files.append(path)
            elif check_subs and isdir(path):
                files += getFiles(path, check_subs, ignored_dirs)
    else:
        Log.d(f'Ignoring {root}, found in {ignored_dirs}')

    return files

def isFileTracked(filename, timeout):
    """
    @brief  Identifies whether a file is tracked in git
    @param  filename    The file to be checked
    @param  timeout     The length of time to wait for the git command
    @return Whether the file is tracked
    """
    args = ['git', 'ls-files', '--error-unmatch', filename]
    p = Popen(args, stdout=PIPE, stderr=PIPE)
    p.communicate(timeout=timeout)
    tracked = False
    if p.returncode == 0:
        tracked = True
    elif p.returncode != 1:
        Log.w(f'Error accessing Git repository in {dirname(filename)}')
    return tracked


def isFileChangedInGit(filename, timeout):
    """
    @brief  Identifies whether a file has been updated on the current working
            directory
    @param  filename    The file to be checked
    @param  timeout     The length of time to wait for the git command
    @return Whether the file has changed in git
    """
    args = ['git', 'diff-index', '--quiet', 'HEAD', '--', filename]
    p = Popen(args, stderr=PIPE)
    o, e = p.communicate(timeout=timeout)
    return p.returncode == 1

def getGitDirectory(filename, timeout):
    """
    Gets the top-level directory name for the given file
    @param  filename    The file within the git repository
    @param  timeoput    The length of time to wait for the git command
    @return The path to the top level directory
    """
    args = ['git', 'rev-parse', '--show-toplevel']
    p = Popen(args, stdout=PIPE, stderr=PIPE)
    o, e = p.communicate(timeout=timeout)
    return o.decode('utf-8').strip() if p.returncode == 0 else None

def commitChanges(repo, files, commit_message, timeout):
    """
    Triggers a commit to a given repo
    @param  repo            The path to the git repository
    @pram   files           The collection of files to add to the commit
    @param  commit_message  The commit message to use
    @param  timeout         The length of time to wait for trivial git commands
    """
    if isinstance(files, str):
        files = [files]
    if not isinstance(files, list):
        raise NotImplementedError('Files must be provided as a list or string')
    original_directory = getcwd()
    # Move into the git repository's directory
    chdir(repo)
    args = ['git', 'add'] + files
    p = Popen(args)
    o, e = p.communicate(timeout=timeout)
    if p.returncode != 0:
        Log.e('Failed to add files')
    else:
        args = ['git', 'commit']
        if commit_message is not None:
            args += ['-m', commit_message]
        p = Popen(args)
        # Note: No timeout here, we are waiting on the user to fill in the
        #       commit message.
        o, e = p.communicate()

    # Return to the previous directory
    chdir(original_directory)

    return p.returncode == 0

#
# The main functionality of the script
#
if __name__ == '__main__':
    #
    # Process arguments
    #
    args = parser.parse_args()
    if args.code:
        code_parser.print_help()
        exit(0)

    # Check for no file or directories provided and set default
    if len(args.files) == 0 and len(args.directories) == 0:
        args.directories = [getcwd()]

    # Add to excluded files
    if not args.include_self:
        args.exclude.append(dirname(realpath(__file__)))

    # Get correct directories for individual files provided
    for i in range(len(args.files)):
        args.files[i] = join(getcwd(), args.files[i])

    #
    # Set up logging
    #
    TRACKED_TYPE = 'tracked'
    Log.set_verb(Log.VERB_WARNING if args.quiet else Log.VERB_INFO)
    Log.set_log(TRACKED_TYPE, colour=Log.COL_YLW, prefix='')
    Log.set_info(prefix='')

    #
    # Gather files in requested directories and sub-directories
    #
    for d in [realpath(join(getcwd(), d)) for d in args.directories]:
        Log.i(f'Checking {d}', end="")
        if args.sub:
            Log.i(' and sub-directories')
        else:
            Log.i('')

        if exists(d) and isdir(d):
            Log.d(f'Directory Valid: {d}')
            args.files += getFiles(d, args.sub, args.exclude)

    #
    # Parse files
    #
    files_changed = []
    original_directory = getcwd();
    file_parser = FileToFileParser()
    for i, file in enumerate(args.files):
        if isfile(file):
            progress = 100. * float(i + 1) / float(len(args.files))
            chdir(dirname(file))
            Log.i(f"Parsing: [{round(progress)}%] {file}")
            if file_parser.parse(file, args.backup):
                files_changed.append(file)
    chdir(original_directory)

    #
    # Report changed files
    #
    repsitories_changed = {}
    original_directory = getcwd();
    tracked_changes = []
    Log.d(f'There are {len(files_changed)} files changed')
    if len(files_changed) > 0:
        # We need to recover a new line after the parsing progress, so \n at start
        Log.i('\nFiles updated on this run:')
        for file in files_changed:
            Log.i('\t' + file)
            chdir(dirname(file))
            if isFileTracked(file, args.timeout) and isFileChangedInGit(file, args.timeout):
                tracked_changes.append(file)
                # Gather list of repositories with changes
                repo = getGitDirectory(file, args.timeout)
                if repo not in repsitories_changed:
                    repsitories_changed[repo] = []
                repsitories_changed[repo].append(file)

    if not args.ignore_git and len(tracked_changes) > 0:
        Log.message(TRACKED_TYPE, 'Files tracked by Git modified on this run:')
        for file in tracked_changes:
            Log.message(TRACKED_TYPE, '\t' + file)

    chdir(original_directory)

    # Commit arguments can be None or a string - When not in use, empty string
    if args.commit != "":
        for repo, files in repsitories_changed.items():
            Log.i(f'Committing changes to repository: {repo}')
            commitChanges(repo, files, args.commit, args.timeout)


    if not args.ignore_untracked:
        exit(len(files_changed))
    elif not args.ignore_git:
        exit(len(tracked_changes))

    exit(0)
