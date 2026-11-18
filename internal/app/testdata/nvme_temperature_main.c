static void reset_state(int available, int error, int prior_smart) {
  hid_available = available;
  enumeration_error = error;
  g_nvme_temp_count = prior_smart;
  g_nvme_smart_active = prior_smart;
  memset(g_nvme_temps, 0, sizeof(g_nvme_temps));
  if (prior_smart) g_nvme_temps[0].value = 55;
}

int main(int argc, char **argv) {
  if (argc != 2) return 2;
  const void *service = CFSTR("fake-service");
  services = CFArrayCreate(NULL, &service, 1, &kCFTypeArrayCallBacks);
  int unavailable = strcmp(argv[1], "both-unavailable") == 0;
  int prior = unavailable || strcmp(argv[1], "smart-to-hid") == 0;
  reset_state(!unavailable, strcmp(argv[1], "empty") != 0, prior);
  for (int iteration = 0; iteration < 10; iteration++) {
    sample_test_temperatures();
    int want_count = unavailable ? 0 : 1;
    if (g_nvme_temp_count != want_count || g_nvme_smart_active != 0 ||
        (!unavailable && (g_nvme_temps[0].value != 42 ||
         strcmp(g_nvme_temps[0].key, "Nv00") != 0 ||
         strcmp(g_nvme_temps[0].name, "NVMe Apple NAND temp") != 0))) {
      fprintf(stderr, "%s iteration=%d count=%d active=%d value=%g\n",
              argv[1], iteration, g_nvme_temp_count, g_nvme_smart_active,
              g_nvme_temp_count ? g_nvme_temps[0].value : 0);
      return 1;
    }
  }
  CFRelease(services);
  return 0;
}
