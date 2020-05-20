[![programming.in.th](https://raw.githubusercontent.com/programming-in-th/artworks/master/png/readme_banner.png)](https://betabeta.programming.in.th)

# Programming.in.th Grader

WIP

## Directories
The general directory hierarchy of the grader is as follows:
- Base directory
    - globalConfig.json
    - defaultCheckers (directory)
    - defaultGroupers (directory)
    - Task 1 (directory)
        - manifest.json
        - compileFiles (directory)
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
* globalConfig.json: contains configuration data that persists across all tasks and shell commands needed to compile user programs (see Global Configuration)
* defaultCheckers: a directory containing executables which are the checker scripts provided by the grader. For most tasks, it is suitable to use one of the checkers contained in this folder. For more details, see Default Checkers.
* defaultGroupers: a directory containing executables which are the groupers provided by the grader. For most tasks, it is suitable to use one of the groupers contained in this folder. For more details, see Default Groupers.

* manifest.json: contains all the meta data about a task (see Manifest Format)
* compileFiles: contains source files that will be compiled alongside the user's source file (used for most non-batch tasks)
* inputs: stores input files for each test case. Each file must be of the form 1.in, 2.in, etc. indicating the index of each test case.
* solutions: stores solution files for each test case. Each file must be of the form 1.sol, 2.sol, etc. indicating the index of each test case.
* checker (optional): an custom executable checker script (see Checker)
* grouper (optional): an custom executable grouper to compute the scores for each group based off of checker outputs (see Grouper)

**Remark 1:** outputs and user\_bin directories do not need to be manually created since the grader automatically creates these if they don't exist.

**Remark 2:** the manifest file must be stored in the root of the task's directory and have the exact name manifest.json" (without quotes). The same applies to the custom checker and/or grouper if included, which must have the names "checker" and "grouper" respectively. Furthermore, both the checker and group script must have executable permissions (you can add them with chmod +x).

## Global Configuration
The global configuration is stored in the file globalConfig.json at the root of the base directory. The shell commands used to compile the user's program are stored in the CompileConfiguration field, whose value is a map keyed by language. Furthermore, since multiple compile commands for different versions of the same language are allowed (and count as different languages), the file extension for each language must also be specified. The JSON file is an array objects containing the following fields:

**IMPORTANT:** You must still specify the extension for interpreted languages (such as Python) and omit the CompileCommands field from the language's corresponding object.

* ID: indicates the ID of the language. These must be unique, as different versions of the same language will be identified by this ID later in the manifest file (see Manifest Format).
* Extension: indicates the corresponding file extension for the language
* CompileCommands: an array of strings indicating the commands and arguments tot be run in the shell for compiling the user's source code, each string being individual tokens in the command.

To denote the user's source code, simply add "\$SRC" as an element in the array. Note that if there are any library files specified in CompileFiles in manifest.json (see Manifest Format), they will be inserted into the command where "\SRC" is as a space-separated string along with the path to the user's source code. You must also add "\$BIN" in the array to denote the argument that indicates the path to the output executable.

The default message to display in the last line of the checker's output for the "Correct" and "Incorrect" verdicts can be configured in the DefaultMessages field, which contains a map that only has the keys "Correct" and "Incorrect".

A sample global configuration is as follows:
```json
{
    "CompileConfiguration": [
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
    ],
    "DefaultMessages": {
        "Correct": "Output is correct",
        "Incorrect": "Output is incorrect"
    }
}
```

## Manifest Format

The manifest file for each task is stored in the JSON format as "manifest.json", and placed at the root of the task's directory.

Optional fields below do not need to be included in every manifest file, depending on the desired functionality. However, non-optional fields must be present in every manifest file.

All fields are required, which include:
* ID: A string indicating the task ID. Must match the task's directory name.
* DefaultLimits: An object storing the default time limit and memory limit for this task. For any supported language (as specified in Compile Configuration) that is not specified in the Limits field below, the default time limit and memory limit will be used.
    * TimeLimit: A floating-point number indicating the time limit of the task in seconds
    * MemoryLimit: An integer indicating the memory limit of the task in MB
* Limits (optional): An object storing custom time limits and memory limits for each language. Each key is a language specified in the Global Configuration and each value is an object having the same TimeLimit and MemoryLimit fields as above. These settings can be used in conjunction with DefaultLimits, as it overrides the time limit and memory limit set in DefaultLimits. See remark below for more details.
* Checker: the name of the checker script to use. These are simply the file names of the default checkers stored in the defaultCheckers directory. If a custom checker is to be used, this value should be set to "custom".
* Grouper: the name of the grouper script to use. Similarly, these are the file names of the default grouprs stored in the defualtGroupers directory. If a custom grouper is to be used, this value should be set to "custom".
* Groups: An array of objects, each denoting one test group. Each group has the following properties:
    * FullScore: A floating-point number indicating the full score of that test group
    * Dependencies (optional): An array of integers indicating the indices of test groups that have to be passed (full score must be achieved) before any score can be gained from the current test group. All indices in the array must be less than the current test group index.
    * TestIndices: An object that indicates the continuous range of indices of tests in TestInputs and TestSolutions that belong to the test group (**test indices start at 1**)
        * Start: An integer denoting the starting index of the test index range (**inclusive**)
        * End: An integer denoting the ending index of the test index range (**inclusive**)
* CompileFiles (optional): An object indicating the files to compile alongside the user's source code for each language (mostly for interactive/communication tasks). Each key is a language specified in the Global Configuration. Corresponding values are arrays of strings, containing the paths of each file **relative to the compileFiles directory**

**Remark:** if any language has its limits set explicitly to null, then the grader will reject all submissions of that language. Note that this is different from not including information about that language at all (i.e. the corresponding language's limits will be undefined rather than null). If DefaultLimits is undefined or null, then only languages supported for this task are those specified as keys here in Limits (non-undefined values) and have non-null values.


Here is a sample manifest.json file:
```json
{
    "ID": "rectsum",
    "DefaultLimits": { "TimeLimit": 10, "MemoryLimit": 256 },
    "Limits": {
        "python3": { "TimeLimit": 20, "MemoryLimit": 256 },
        "java8": null
    },
    "Checker": "custom",
    "Grouper": "min",
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

To illustrate the concept of the manifest file better, consider the above sample. The default limits are 10 seconds and 256 MB for all languages except for Python and Java. Since Python is a slow language, we set the time limit for Python at 20 seconds instead, and explicity disallow Java submissions. We see that the first test group contains tests with indices from 1 to 15 (inclusive) and has no dependencies on any other test groups. On the other hand the second test group is comprised of test with indices from 16 to 20 (inclusive) and the user can only score more than 0 points on this test group if the first test group is passed. For C++, we have extra files to compile alongside the user's source code, namely "joi.h" and "joi.cpp" repsectively.

**Note:** for output-only tasks, omit the DefaultLimits field (or set it to null) and use the following configuration for the Limits field:

```json
"Limits": {
    "text": {}
}
```

## Checker
The checker script is run for each test case and the results are stored as plain text in /tmp/grader/{submissionID}/{testCaseIndex}.check, where {submissionID} and {testCaseIndex} are placeholders for the submission ID and current test case index respectively. The grouper will then read from these files to determine the scores for each test group.

The checker script must be provided by the user and takes in the following command-line arguments:
1. **Absolute** path to the input file of the test case
2. **Absolute** path to the output file generated by the user's program for the test case
3. **Absolute** path to the solution file of the test case

Of course, the checker script is passed to itself as the 0-th argument, but it can be safely ignored.

The checker must then write two lines to standard output. The first line denotes the verdict of the user's program, which one be either of the following:

* Correct
* Partially correct
* Incorrect

The score of the user's program on the current test case must then be printed on the second line. If the user receives a "Correct" or "Partially correct" verdict on the current test case, then the score can be **any real floating-point number**. Otherwise, if the user receives an "Incorrect" verdict, then the score on the second line of the custom checker's output must be 0. Finally, on the last line, the checker can **optionally** output a message describing the result of the test case. If no message is provided, the default message specified in the global configuration (see Global Configuration) will be automatically added instead.

For example,

```plaintext
Correct
20
Target reached in 25 moves
```
,

```plaintext
Partially Correct
17.5
Target reached in 30 moves
```

and

```plaintext
Incorrect
0
Wrong format
```

are valid outputs from the checker.

In the case when the program exceeds the time limit, memory limit, or encounters a runtime error, the grader will write the following to the /tmp/grader/{submissionID}/{testCaseIndex}.check instead of running the checker script. You **must not** handle this manually. In other words, the following verdicts **must not** be used by a custom checker.

Time Limit Exceeded:
```plaintext
Time limit exceeded
0
Process killed: wall time limit exceeded
```

Memory Limit Exceeded:
```plaintext
Memory limit exceeded
0
Process killed: max-rss exceeds memory limit
```

Runtime error:
```plaintext
Runtime error
0
Process killed: runtime error
```

## Default Checkers

## Grouper
The grouper script's role is to gather individual scores and verdicts from the checker to determine the score on a test group. Note that the grouper does not handle dependencies between test groups, as that is already handled automatically by the grader via manifest.json. Hence, the grouper will run once per test group. Note that we provide some groupers for normal use cases, but you may decide to write your own grouper if you need more sophisticated custom functionality.

The grouper must three command line arguments (excluding the name of the grouper itself):
1. A floating-point number indicating the maximum score of the test group
2. An integer indicating the starting test index of the test group (1-indexed)
3. An integer indicating the ending (inclusive) test index of the test group (1-indexed)

The grouper must then print the score of the test group to standard output as a floating point number.

Note that the grouper should access /tmp/grader/{submissionID}/{testIndex}.check for test index within the range specified by the command line arguments to determine the score. {submissionID} and {testIndex} are placeholders for the current submission ID and test index respectively.

### Default Groupers
A default grouper is provided which can be used with the majority of tasks. As stated in the Checker section, the grouper accepts scores on tests out of 10 relative to the full score of the test group. It will then scale the score out of 10 by the full score of the test group, and take the **minimum** score among all tests in the test group and print it to standard output.
