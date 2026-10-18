#include "create.hpp"

#include "util.hpp"
#include "version.hpp"

#include <libtorrent/add_torrent_params.hpp>
#include <libtorrent/create_torrent.hpp>
#include <libtorrent/load_torrent.hpp>
#include <libtorrent/span.hpp>
#include <libtorrent/torrent_info.hpp>
#include <nlohmann/json.hpp>

#include <exception>
#include <filesystem>
#include <iostream>

namespace odm {

namespace {

int usage() {
  std::cerr << "usage: odm-libtorrent create <file or folder> <out.torrent> [--piece-size <bytes>]\n"
               "                             [--tracker <url>]... [--private] [--v1-only]\n";
  return 2;
}

}  // namespace

int create_main(std::vector<std::string> const& args) {
  std::vector<std::string> paths;
  std::vector<std::string> trackers;
  int piece_size = 0;
  bool priv = false;
  lt::create_flags_t flags{};
  for (std::size_t i = 1; i < args.size(); ++i) {
    std::string const& a = args[i];
    if (a == "--piece-size" && i + 1 < args.size()) {
      piece_size = std::stoi(args[++i]);
    } else if (a == "--tracker" && i + 1 < args.size()) {
      trackers.push_back(args[++i]);
    } else if (a == "--private") {
      priv = true;
    } else if (a == "--v1-only") {
      flags |= lt::create_torrent::v1_only;
    } else if (!a.empty() && a[0] == '-') {
      return usage();
    } else {
      paths.push_back(a);
    }
  }
  if (paths.size() != 2) return usage();
  try {
    namespace fs = std::filesystem;
    fs::path src = fs::absolute(fs::u8path(native(paths[0])));
    // The torrent is named after src; its files' paths start with that name
    // for a folder, and hashing reads them from src's parent.
    lt::create_torrent ct(lt::list_files(src.u8string(), flags), piece_size, flags);
    for (auto const& t : trackers) ct.add_tracker(t);
    ct.set_priv(priv);
    std::string creator = std::string("odm-libtorrent ") + ODM_LIBTORRENT_VERSION;
    ct.set_creator(creator.c_str());
    lt::set_piece_hashes(ct, src.parent_path().u8string());
    std::vector<char> buf = ct.generate_buf();
    std::string err;
    if (!write_file(paths[1], buf, err)) {
      std::cerr << "odm-libtorrent create: " << err << '\n';
      return 1;
    }
    lt::add_torrent_params atp =
        lt::load_torrent_buffer(lt::span<char const>(buf.data(), static_cast<std::ptrdiff_t>(buf.size())));
    lt::info_hash_t ih = atp.ti->info_hashes();
    nlohmann::json out = nlohmann::json::object();
    if (ih.has_v1()) out["info_hash"] = hex(ih.v1.data(), ih.v1.size());
    if (ih.has_v2()) out["info_hash_v2"] = hex(ih.v2.data(), ih.v2.size());
    std::cout << out.dump() << '\n';
    return 0;
  } catch (std::exception const& e) {
    std::cerr << "odm-libtorrent create: " << e.what() << '\n';
    return 1;
  }
}

}  // namespace odm
