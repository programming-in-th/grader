#include "testlib.h"

using namespace std;

string upper(string sa) {
  for (size_t i = 0; i < sa.length(); i++)
    if ('a' <= sa[i] && sa[i] <= 'z')
      sa[i] = sa[i] - 'a' + 'A';
  return sa;
}

int main(int argc, char *argv[]) {
  inf.init(argv[1], _input);
  ouf.init(argv[2], _output);
  ans.init(argv[3], _answer);

  string ja = upper(ans.readWord());
  string pa = upper(ouf.readWord());

  if ((pa != "YES" && pa != "NO") || (ja != "YES" && ja != "NO") || ja != pa) {
    cout << "Incorrect" << endl << "0" << endl;
    return 0;
  }
  
  cout << "Correct" << endl << "100" << endl;
  return 0;
}