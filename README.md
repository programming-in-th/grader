[![programming.in.th](https://raw.githubusercontent.com/programming-in-th/artworks/master/png/readme_banner.png)](https://betabeta.programming.in.th)

# Programming.in.th Grader

## Directories
The general directory hierarchy of the grader is as follows:
- Base directory
    - compileConfig.json
    - Task 1 (directory)
        - manifest.json
        - inputs (directory)
        - solutions (directory)
        - checker
        - grouper
    - Task 2 (directory)
        - manifest.json
        ...
    - Task 3 (directory)
        ...

What does each directory/file do?
* compileConfig.json: contains shell commands needed to compile user programs (see Compile Configuration)
* manifest.json: contains all the meta data about a task (see Manifest Format)
* inputs: stores input files for each test case. Each file must be of the form 1.in, 2.in, etc. indicating the index of each test case.
* solutions: stores solution files for each test case. Each file must be of the form 1.sol, 2.sol, etc. indicating the index of each test case.
* checker: an executable checker script (see Checker)
* grouper: an executable to compute the scores for each group based off of checker outputs (see Grouper)

**Remark 1:** outputs and user\_bin directories do not need to be manually created since the grader automatically creates these if they don't exist.

**Remark 2:** the checker script, grouper script and manifest file must be stored in the root of the task's directory and have the exact names "checker", "grouper" and "manifest.json" (without quotes). Furthermore, both the checker and group script must have executable permissions (you can add them with chmod +x)

## Compile Configuration
The compile configuration is stored in the file compileConfig.json at the root of the base directory. The shell commands used to compile the user's program are stored in a map, keyed by language. Furthermore, since multiple compile commands for different versions of the same language are allowed (and count as different languages), the file extension for each language must also be specified. The JSON file is an array objects containing the following fields:

**IMPORTANT:** You must still specify the extension for interpreted languages (such as Python) and omit the CompileCommands field from the language's corresponding object.

* ID: indicates the ID of the language. These must be unique, as different versions of the same language will be identified by this ID later in the manifest file (see Manifest Format).
* Extension: indicates the corresponding file extension for the language
* CompileCommands: an array of strings indicating the commands and arguments tot be run in the shell for compiling the user's source code, each string being individual tokens in the command.

To denote the user's source code, simply add "\$SRC" as an element in the array. Note that if there are any library files specified in CompileFiles in manifest.json (see Manifest Format), they will be inserted into the command where "\SRC" is as a space-separated string along with the path to the user's source code. You must also add "\$BIN" in the array to denote the argument that indicates the path to the output executable.

A sample compile configuration is as follows:
```json
[
    {
        "ID": "cpp14",
        "Extension": "cpp",
        "CompileCommands": ["/usr/bin/c++", "--std=c++14", "$SRC"]
    },
    {
        "ID": "python3",
        "Extension": "py"
    },
    {
        "ID": "python2",
        "Extension": "py"
    }
]
```

## Manifest Format

The manifest file for each task is stored in the JSON format as "manifest.json", and placed at the root of the task's directory.

Optional fields below do not need to be included in every manifest file, depending on the desired functionality. However, non-optional fields must be present in every manifest file.

All fields are required, which include:
* ID: A string indicating the task ID. Must match the task's directory name.
* DefaultLimits: An object storing the default time limit and memory limit for this task. For any supported language (as specified in Compile Configuration) that is not specified in the Limits field below, the default time limit and memory limit will be used.
    * TimeLimit: A floating-point number indicating the time limit of the task in seconds
    * MemoryLimit: An integer indicating the memory limit of the task in KB
* Limits (optional): An object storing custom time limits and memory limits for each language. Each key is a language specified in the Global Configuration and each value is an object having the same TimeLimit and MemoryLimit fields as above. These settings can be used in conjunction with DefaultLimits, as it overrides the time limit and memory limit set in DefaultLimits. See remark below for more details.
* Groups: An array of objects, each denoting one test group. Each group has the following properties:
    * FullScore: A floating-point number indicating the full score of that test group
    * Dependencies (optional): An array of integers indicating the indices of test groups that have to be passed (full score must be achieved) before any score can be gained from the current test group. All indices in the array must be less than the current test group index.
    * TestIndices: An object that indicates the continuous range of indices of tests in TestInputs and TestSolutions that belong to the test group (**test indices start at 1**)
        * Start: An integer denoting the starting index of the test index range (**inclusive**)
        * End: An integer denoting the ending index of the test index range (**inclusive**)
* CompileFiles (optional): An object indicating the files to compile alongside the user's source code for each language (mostly for interactive/communication problems). Each key is a language specified in the Global Configuration. Corresponding values are arrays of strings, containing the paths of each file **relative to the task's directory**

**Remark:** if any language has its limits set explicitly to null, then the grader will reject all submissions of that language. Note that this is different from not including information about that language at all (i.e. the corresponding language's limits will be undefined rather than null). If DefaultLimits is undefined or null, then only languages supported for this task are those specified as keys here in Limits (non-undefined values) and have non-null values.


Here is a sample manifest.json file:
```json
{
    "ID": "rectsum",
    "DefaultLimits": { "TimeLimit": 10, "MemoryLimit": 256000 },
    "Limits": {
        "python3": { "TimeLimit": 20, "MemoryLimit": 256000 },
        "java8": null
    },
    "Groups": [
        {
            "FullScore": 29,
            "TestIndices": {
            "Start": 1,
            "End": 15
            }
        },
        {
            "FullScore": 71,
            "Dependencies": [1],
            "TestIndices": {
                "Start": 16,
                "End": 20
            }
        }
    ],
    "CompileFiles": {
        "cpp14": ["joi.h", "joi.cpp"]
    }
}
```

To illustrate the concept of the manifest file better, consider the above sample. The default limits are 10 seconds and 256000 KB for all languages except for Python and Java. Since Python is a slow language, we set the time limit for Python at 20 seconds instead, and explicity disallow Java submissions. We see that the first test group contains tests with indices from 1 to 15 (inclusive) and has no dependencies on any other test groups. On the other hand the second test group is comprised of test with indices from 16 to 20 (inclusive) and the user can only score more than 0 points on this test group if the first test group is passed. For C++, we have extra files to compile alongside the user's source code, namely "joi.h" and "joi.cpp" repsectively.

**Note:** for output-only problems, omit the DefaultLimits field (or set it to null) and use the following configuration for the Limits field:
```json
"Limits": {
    "text": {}
}
```

## Checker
The checker script is run for each test case and the results are stored as plain text in /tmp/{submissionID}/{testCaseIndex}.check, where {submissionID} and {testCaseIndex} are placeholders for the submission ID and current test case index respectively. The grouper will then read from these files to determine the scores for each test group.

The checker script must be provided by the user and takes in the following command-line arguments:
1. **Absolute** path to the input file of the test case
2. **Absolute** path to the output file generated by the user's program for the test case
3. **Absolute** path to the solution file of the test case

Of course, the checker script is passed to itself as the 0-th argument, but it can be safely ignored.

The checker must then write two lines to standard output. The first line denotes whether or not the user should receive the "Correct" verdict on the test case. If "Correct" is printed, then the user will be judged as "Correct" on the test case. Otherwise, if "Incorrect" is printed, then the user will received a "Wrong Answer" verdict. The second line must include a floating-point number indicating the user's score on the test case. If the "Correct" verdict is given on the first line, then any real-numbered score is valid. Otherwise, if the "Incorrect" verdict is given on the first line, then the checker must print 0 on the second line. Finally, on the last line, the checker can **optionally** output a message describing the result of the test case.

For example,

```plaintext
Correct
75
Target reached in 25 moves
```

and

```plaintext
Incorrect
0
Wrong format
```

are valid outputs from the checker.

**However**, note that the default checker (packaged with the grader) requires that the checker's output be a **percentage** of the test case's full score. Thus, when using one of the groupers packaged with the grader, be aware what their specifications are.

## Grouper
The grouper script's role is to gather individual scores and verdicts from the checker to determine the score on a test group. Note that the grouper does not handle dependencies between test groups, as that is already handled automatically by the grader via manifest.json. Hence, the grouper will run once per test group. Note that we provide some groupers for normal use cases, but you may decide to write your own grouper if you need more sophisticated custom functionality.

The grouper must three command line arguments (excluding the name of the grouper itself):
1. A floating-point number indicating the maximum score of the test group
2. An integer indicating the starting test index of the test group (1-indexed)
3. An integer indicating the ending (inclusive) test index of the test group (1-indexed)

The grouper must then print the score of the test group to standard output as a floating point number.

Note that the grouper should access /tmp/{submissionID}/{testIndex}.check for test index within the range specified by the command line arguments to determine the score. {submissionID} and {testIndex} are placeholders for the current submission ID and test index respectively.
