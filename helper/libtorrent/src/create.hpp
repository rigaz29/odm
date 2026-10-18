// odm-libtorrent create: makes a .torrent file, for tests.
#pragma once

#include <string>
#include <vector>

namespace odm {

// create_main runs "odm-libtorrent create"; args are the program's
// arguments, args[0] being "create".
int create_main(std::vector<std::string> const& args);

}  // namespace odm
