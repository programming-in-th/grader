#include "testlib.h"
#include <sstream>

using namespace std;

string ending(long long x) {
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

string ltoa(long long n) {
  stringstream ss;
  ss << n;
  string result;
  ss >> result;
  return result;
}

int main(int argc, char *argv[]) {
  inf.init(argv[1], _input);
  ouf.init(argv[2], _output);
  ans.init(argv[3], _answer);

  int n = 0;

  string firstElems;

  while (!ans.seekEof() && !ouf.seekEof()) {
    n++;
    long long j = ans.readLong();
    long long p = ouf.readLong();
    if (j != p) {
      cout << "Incorrect" << endl << "0" << endl;
      return 0;
    } else if (n <= 5) {
      if (firstElems.length() > 0)
        firstElems += " ";
      firstElems += ltoa(j);
    }
  }

  int extraInAnsCount = 0;

  while (!ans.seekEof()) {
    ans.readLong();
    extraInAnsCount++;
  }

  int extraInOufCount = 0;

  while (!ouf.seekEof()) {
    ouf.readLong();
    extraInOufCount++;
  }

  if (extraInAnsCount > 0 || extraInOufCount > 0) {
    cout << "Incorrect" << endl << "0" << endl;
    return 0;
  }

  cout << "Correct" << endl << "100" << endl;
  return 0;
}