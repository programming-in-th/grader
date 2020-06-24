#include "testlib.h"
#include <cmath>

using namespace std;

#define EPS 1E-9

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
  
  double j, p;

  while (!ans.seekEof()) {
    j = ans.readDouble();
    p = ouf.readDouble();
    if (!doubleCompare(j, p, EPS)) {
      cout << "Incorrect" << endl << "0" << endl;
      return 0;
    }
  }

  cout << "Correct" << endl << "100" << endl;
  return 0;
}