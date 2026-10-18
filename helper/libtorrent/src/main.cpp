// odm-libtorrent: the companion program Ochi Download Manager runs for
// torrents, on libtorrent. ODM writes commands to its stdin and reads replies
// and events from its stdout, one JSON object a line (PROTOCOL.md).
//
//   odm-libtorrent [--state <file>]
//   odm-libtorrent create <file or folder> <out.torrent> [options]
//   odm-libtorrent --version
#include "create.hpp"
#include "helper.hpp"
#include "util.hpp"
#include "version.hpp"

#include <libtorrent/version.hpp>

#include <csignal>
#include <cstdio>
#include <cstdlib>
#include <exception>
#include <iostream>
#include <string>
#include <vector>

#ifdef _WIN32
#include <fcntl.h>
#include <io.h>
#endif

int main(int argc, char* argv[]) {
#ifdef _WIN32
  // Lines are UTF-8 ending in \n, not the console's code page and \r\n.
  _setmode(_fileno(stdin), _O_BINARY);
  _setmode(_fileno(stdout), _O_BINARY);
#else
  // A write to an ODM that is gone fails instead of killing the program.
  std::signal(SIGPIPE, SIG_IGN);
#endif
  std::vector<std::string> args = odm::args(argc, argv);
  if (!args.empty() && args[0] == "--version") {
    std::cout << "odm-libtorrent " << ODM_LIBTORRENT_VERSION << " (protocol " << odm::kProtocol
              << ", libtorrent " << LIBTORRENT_VERSION << ")\n";
    return 0;
  }
  if (!args.empty() && args[0] == "create") return odm::create_main(args);

  std::string state;
  for (std::size_t i = 0; i < args.size(); ++i) {
    if (args[i] == "--state" && i + 1 < args.size()) {
      state = odm::native(args[++i]);
    } else {
      std::cerr << "usage: odm-libtorrent [--state <file>]\n"
                   "       odm-libtorrent create <file or folder> <out.torrent> [options]\n"
                   "       odm-libtorrent --version\n";
      return 2;
    }
  }
  int code = 1;
  try {
    odm::Helper helper(state);
    code = helper.run();
  } catch (std::exception const& e) {
    std::cerr << "odm-libtorrent: " << e.what() << '\n';
  }
  // Everything is saved. Leave without exit()'s clean-up: the thread that
  // reads stdin may still be waiting in it after "quit", and glibc 2.39's
  // exit() waits for that stream's lock, so the program never ended.
  std::cout.flush();
  std::fflush(nullptr);
  std::_Exit(code);
}
