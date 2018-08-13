#pragma once

#include <string>
#include "bootimg.h"

class img_exception : public std::runtime_error {
public:
    img_exception(const std::string &what) : std::runtime_error(what) {}
};

class Image {
public:
    void decompress_ramdisk(char comp_mode);

    boot_img_hdr hdr;

    std::shared_ptr<std::string> kernel;
    std::shared_ptr<std::string> ramdisk;
    std::shared_ptr<std::string> second;
    std::shared_ptr<std::string> device_tree;

private:
    void decompress_ramdisk_gzip();
    void decompress_ramdisk_lzo();
    void decompress_ramdisk_xz();
};