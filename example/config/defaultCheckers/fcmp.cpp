#include "testlib.h"
#include <string>
#include <vector>
#include <sstream>

using namespace std;

string ending(int x) {
  x %= 100;
  if (x / 10 == 1)
    return "th";
  if (x % 10 == 1)
    return "st";
  if (x % 10 == 2)
    return "nd";
  if (x % 10 == 3)
    return "rd";
  return "th";
}

int main(int argc, char *argv[]) {
  inf.init(argv[1], _input);
  ouf.init(argv[2], _output);
  ans.init(argv[3], _answer);

  string strAnswer;

  while (!ans.eof()) {
    string j = ans.readString();

    if (j == "" && ans.eof())
      break;

    strAnswer = j;
    string p = ouf.readString();

    if (j != p) {
      cout << "Incorrect" << endl << "0" << endl;
      return 0;
    }
  }

  cout << "Correct" << endl << "100" << endl;
  return 0;
}