#include "testlib.h"
#include <string>
#include <vector>
#include <sstream>

using namespace std;

bool compareWords(string a, string b) {
    vector<string> va, vb;
    stringstream sa;
    
    sa << a;
    string cur;
    while (sa >> cur)
        va.push_back(cur);

    stringstream sb;
    sb << b;
    while (sb >> cur)
        vb.push_back(cur);

    return (va == vb);
}

int main(int argc, char * argv[]) {
    inf.init(argv[1], _input);
    ouf.init(argv[2], _output);
    ans.init(argv[3], _answer);

    std::string strAnswer;

    while (!ans.eof()) {
        string jj = ans.readString();

        if (jj == "" && ans.eof())
          break;
        
        string pp = ouf.readString();
        strAnswer = pp;

        if (!compareWords(jj, pp)) {
          cout << "Incorrect" << endl << "0" << endl;
          return 0;
        }
    }
    
    cout << "Correct" << endl << "100" << endl;
    return 0;
}