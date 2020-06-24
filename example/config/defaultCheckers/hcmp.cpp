#include "testlib.h"
#include <string>

using namespace std;

string part(const string &s) {
  if (s.length() <= 128)
    return s;
  else
    return s.substr(0, 64) + "..." + s.substr(s.length() - 64, 64);
}

bool isNumeric(string p) {
  bool minus = false;

  if (p[0] == '-')
    minus = true, p = p.substr(1);

  for (int i = 0; i < p.length(); i++)
    if (p[i] < '0' || p[i] > '9')
      return false;

  if (minus)
    return (p.length() > 0 && (p.length() == 1 || p[0] != '0')) &&
           (p.length() > 1 || p[0] != '0');
  else
    return p.length() > 0 && (p.length() == 1 || p[0] != '0');
}

int main(int argc, char *argv[]) {
  inf.init(argv[1], _input);
  ouf.init(argv[2], _output);
  ans.init(argv[3], _answer);

  string ja = ans.readWord();
  string pa = ouf.readWord();

  if (!isNumeric(ja) || !ans.seekEof() || !isNumeric(pa) || ja != pa) {
    cout << "Incorrect" << endl << "0" << endl;
    return 0;
  }

  cout << "Correct" << endl << "100" << endl;
  return 0;
}