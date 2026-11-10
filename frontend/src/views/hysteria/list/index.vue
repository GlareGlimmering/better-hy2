<template>
  <div class="app-container">
    <div class="search">
      <el-form :inline="true">
        <el-form-item>
          <el-button type="primary" @click="submitForm" :icon="Select">
            {{ $t("common.save") }}
          </el-button>
        </el-form-item>
        <el-form-item>
          <el-upload
            v-model:file-list="fileList"
            :http-request="handleImport"
            :show-file-list="false"
            accept=".yaml"
            :limit="1"
            :before-upload="beforeImport"
          >
            <el-button>
              <template #icon>
                <i-ep-upload />
              </template>
              {{ $t("common.import") }}
            </el-button>
          </el-upload>
        </el-form-item>
        <el-form-item>
          <el-button @click="handleExport">
            <template #icon>
              <i-ep-download />
            </template>
            {{ $t("common.export") }}
          </el-button>
        </el-form-item>
        <el-form-item>
          <el-button @click="handleReset">
            <template #icon>
              <i-ep-refresh />
            </template>
            {{ $t("common.reset") }}
          </el-button>
        </el-form-item>
      </el-form>
    </div>

    <el-card shadow="never">
      <el-form inline class="lifecycle-toolbar">
        <el-form-item>
          <el-tag
            :type="hysteria2Monitor.running ? 'success' : 'info'"
            size="large"
          >
            {{
              hysteria2Monitor.running
                ? $t("hysteria.runningState")
                : $t("hysteria.stoppedState")
            }}
          </el-tag>
        </el-form-item>
        <el-form-item v-if="hysteria2Monitor.running">
          <el-button
            :loading="lifecycleLoading"
            :disabled="lifecycleLoading"
            @click="handleLifecycleAction('restart')"
          >
            {{ $t("hysteria.restartService") }}
          </el-button>
          <el-button
            type="danger"
            :loading="lifecycleLoading"
            :disabled="lifecycleLoading"
            @click="handleLifecycleAction('stop')"
          >
            {{ $t("hysteria.stopService") }}
          </el-button>
        </el-form-item>
        <el-form-item v-else>
          <el-button
            type="success"
            :loading="lifecycleLoading"
            :disabled="lifecycleLoading"
            @click="handleLifecycleAction('start')"
          >
            {{ $t("hysteria.startService") }}
          </el-button>
        </el-form-item>
        <el-form-item>
          <div style="display: flex; align-items: center">
            <el-select
              v-model="hysteria2Version"
              :placeholder="$t('hysteria.hysteria2Version')"
              style="height: 32px; width: 160px"
            >
              <el-option
                v-for="item in hysteria2Versions"
                :key="item"
                :label="item"
                :value="item"
              />
            </el-select>
            <el-button
              type="primary"
              @click="handleHysteria2ChangeVersion"
              style="height: 32px"
              >{{ $t("hysteria.hysteria2ChangeVersion") }}
            </el-button>
          </div>
        </el-form-item>
        <el-form-item>
          <el-tag style="height: 32px">
            {{ $t("hysteria.hysteria2Version") }}:
            {{ hysteria2Monitor.version ? hysteria2Monitor.version : "-" }}
          </el-tag>
        </el-form-item>
      </el-form>

      <el-alert
        :title="$t('hysteria.lifecycleNotice')"
        type="info"
        :closable="false"
        show-icon
        style="margin-bottom: 16px"
      />

      <el-alert
        :type="routingStatus.ready ? 'success' : 'warning'"
        :closable="false"
        show-icon
        :title="
          routingStatus.ready
            ? $t('account.routingReady')
            : $t('account.routingNotReady')
        "
        :description="routingStatusDescription"
        style="margin-bottom: 16px"
      />

      <el-form
        ref="dataFormRef"
        :rules="dataFormRules"
        label-position="top"
        :model="dataForm"
      >
        <div class="config-workspace">
          <el-collapse v-model="expandedGroups" class="config-navigation">
            <el-collapse-item :title="$t('hysteria.groups.tls')" name="tls">
              <button type="button" :class="configNavClass('tls')" @click="activeName = 'tls'">{{ $t("hysteria.tls") }}</button>
              <button type="button" :class="configNavClass('ech')" @click="activeName = 'ech'">
                <span>{{ $t("hysteria.ech") }}</span><el-switch v-model="ech" @click.stop @change="handleOptionalToggle('ech', $event)" />
              </button>
            </el-collapse-item>
            <el-collapse-item :title="$t('hysteria.groups.basic')" name="basic">
              <button type="button" :class="configNavClass('extension')" @click="activeName = 'extension'">{{ $t("hysteria.extension") }}</button>
              <button type="button" :class="configNavClass('listen')" @click="activeName = 'listen'">{{ $t("hysteria.listen") }}</button>
            </el-collapse-item>
            <el-collapse-item :title="$t('hysteria.groups.authTraffic')" name="authTraffic">
              <button type="button" :class="configNavClass('http')" @click="activeName = 'http'">{{ $t("hysteria.http") }}</button>
              <p class="managed-config-note">{{ $t("hysteria.managedAuthNotice") }}</p>
            </el-collapse-item>
            <el-collapse-item :title="$t('hysteria.groups.performance')" name="performance">
              <button type="button" :class="configNavClass('bandwidth')" @click="activeName = 'bandwidth'">
                <span>{{ $t("hysteria.bandwidth") }}</span><el-switch v-model="bandwidth" @click.stop @change="handleOptionalToggle('bandwidth', $event)" />
              </button>
              <button type="button" :class="configNavClass('quic')" @click="activeName = 'quic'">
                <span>{{ $t("hysteria.quic") }}</span><el-switch v-model="quic" @click.stop @change="handleOptionalToggle('quic', $event)" />
              </button>
              <button type="button" :class="configNavClass('speedTest')" @click="activeName = 'speedTest'">
                <span>{{ $t("hysteria.speedTest") }}</span><el-switch v-model="speedTest" @click.stop @change="handleOptionalToggle('speedTest', $event)" />
              </button>
            </el-collapse-item>
            <el-collapse-item :title="$t('hysteria.groups.network')" name="network">
              <button type="button" :class="configNavClass('udp')" @click="activeName = 'udp'">
                <span>{{ $t("hysteria.udp") }}</span><el-switch v-model="udp" @click.stop @change="handleOptionalToggle('udp', $event)" />
              </button>
              <button type="button" :class="configNavClass('resolver')" @click="activeName = 'resolver'">
                <span>{{ $t("hysteria.resolver") }}</span><el-switch v-model="resolver" @click.stop @change="handleOptionalToggle('resolver', $event)" />
              </button>
              <button type="button" :class="configNavClass('sniff')" @click="activeName = 'sniff'">
                <span>{{ $t("hysteria.sniff") }}</span><el-switch v-model="sniff" @click.stop @change="handleOptionalToggle('sniff', $event)" />
              </button>
            </el-collapse-item>
            <el-collapse-item :title="$t('hysteria.groups.obfuscation')" name="obfuscation">
              <button type="button" :class="configNavClass('obfs')" @click="activeName = 'obfs'">
                <span>{{ $t("hysteria.obfs") }}</span><el-switch v-model="obfs" @click.stop @change="handleOptionalToggle('obfs', $event)" />
              </button>
              <button type="button" :class="configNavClass('masquerade')" @click="activeName = 'masquerade'">
                <span>{{ $t("hysteria.masquerade") }}</span><el-switch v-model="masquerade" @click.stop @change="handleOptionalToggle('masquerade', $event)" />
              </button>
            </el-collapse-item>
            <el-collapse-item :title="$t('hysteria.groups.advancedRules')" name="advancedRules">
              <button type="button" :class="configNavClass('acl')" @click="activeName = 'acl'">
                <span>{{ $t("hysteria.acl") }}</span><el-switch v-model="acl" @click.stop @change="handleOptionalToggle('acl', $event)" />
              </button>
              <button type="button" :class="configNavClass('outbounds')" @click="activeName = 'outbounds'">
                <span>{{ $t("hysteria.outbounds") }}</span><el-switch v-model="outbounds" @click.stop @change="handleOptionalToggle('outbounds', $event)" />
              </button>
              <p class="managed-config-note">{{ $t("hysteria.managedOutboundsNotice") }}</p>
            </el-collapse-item>
          </el-collapse>
          <el-tabs v-model="activeName" class="tabs config-editor">
          <el-tab-pane :label="$t('hysteria.tls')" name="tls">
            <el-alert v-if="tlsMissing" :title="tlsMissing" type="warning" :closable="false" show-icon class="mb-3" />
            <el-tooltip
              :content="$t('hysteria.config.tlsType')"
              placement="bottom"
            >
              <el-form-item label="tls.type" prop="tls.type">
                <el-select v-model="tlsType" style="width: 100%">
                  <el-option
                    v-for="item in tlsTypes"
                    :key="item"
                    :label="item"
                    :value="item"
                  />
                </el-select>
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="tlsType === 'tls'"
              :content="$t('hysteria.config.tls.cert')"
              placement="bottom"
            >
              <el-form-item label="cert" prop="tls.cert">
                <el-input
                  v-model="dataForm.tls.cert"
                  style="width: 50%"
                  clearable
                />
                <el-upload
                  style="height: 32px"
                  ref="uploadCrtFile"
                  action=""
                  :file-list="crtFileList"
                  :http-request="uploadCertFile"
                  accept=".crt"
                  :before-upload="
                    () => {
                      crtFileList = [];
                    }
                  "
                  :show-file-list="false"
                  :limit="1"
                >
                  <template #trigger>
                    <el-button>{{ t("config.uploadCrtFile") }}</el-button>
                  </template>
                </el-upload>
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="tlsType === 'tls'"
              :content="$t('hysteria.config.tls.key')"
              placement="bottom"
            >
              <el-form-item label="key" prop="tls.key">
                <el-input
                  v-model="dataForm.tls.key"
                  style="width: 50%"
                  clearable
                />
                <el-upload
                  style="height: 32px"
                  ref="uploadKeyFile"
                  action=""
                  :file-list="keyFileList"
                  :http-request="uploadCertFile"
                  accept=".key"
                  :before-upload="
                    () => {
                      keyFileList = [];
                    }
                  "
                  :show-file-list="false"
                  :limit="1"
                >
                  <template #trigger>
                    <el-button>{{ t("config.uploadKeyFile") }}</el-button>
                  </template>
                </el-upload>
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="tlsType === 'tls'"
              :content="$t('hysteria.config.tls.sniGuard')"
              placement="bottom"
            >
              <el-form-item label="sniGuard" prop="tls.sniGuard">
                <el-select
                  v-model="dataForm.tls.sniGuard"
                  style="width: 100%"
                  clearable
                >
                  <el-option
                    v-for="item in tlsSniGuards"
                    :key="item"
                    :label="item"
                    :value="item"
                  />
                </el-select>
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="tlsType === 'acme'"
              :content="$t('hysteria.config.acme.domains')"
              placement="bottom"
            >
              <el-form-item label="acme.domains" prop="acme.domains">
                <imputMultiple :tags="dataForm.acme.domains" />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="tlsType === 'acme'"
              :content="$t('hysteria.config.acme.email')"
              placement="bottom"
            >
              <el-form-item label="acme.email" prop="acme.email">
                <el-input v-model="dataForm.acme.email" clearable />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="tlsType === 'acme'"
              :content="$t('hysteria.config.acme.ca')"
              placement="bottom"
            >
              <el-form-item label="acme.ca" prop="acme.ca">
                <el-select v-model="dataForm.acme.ca" style="width: 100%">
                  <el-option
                    v-for="item in acmeCas"
                    :key="item"
                    :label="item"
                    :value="item"
                  />
                </el-select>
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="tlsType === 'acme'"
              :content="$t('hysteria.config.acme.listenHost')"
              placement="bottom"
            >
              <el-form-item label="acme.listenHost" prop="acme.listenHost">
                <el-input v-model="dataForm.acme.listenHost" clearable />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="tlsType === 'acme'"
              :content="$t('hysteria.config.acme.dir')"
              placement="bottom"
            >
              <el-form-item label="acme.dir" prop="acme.dir">
                <el-input v-model="dataForm.acme.dir" clearable />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="tlsType === 'acme'"
              :content="$t('hysteria.config.acme.type')"
              placement="bottom"
            >
              <el-form-item label="acme.type" prop="acme.type">
                <el-select
                  v-model="dataForm.acme.type"
                  style="width: 100%"
                  clearable
                >
                  <el-option
                    v-for="item in acmeTypes"
                    :key="item"
                    :label="item"
                    :value="item"
                  />
                </el-select>
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="tlsType === 'acme' && dataForm.acme.type === 'http'"
              :content="$t('hysteria.config.acme.http.altPort')"
              placement="bottom"
            >
              <el-form-item label="acme.http.altPort" prop="acme.http.altPort">
                <el-input
                  v-model.number="dataForm.acme.http.altPort"
                  clearable
                />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="tlsType === 'acme' && dataForm.acme.type === 'tls'"
              :content="$t('hysteria.config.acme.tls.altPort')"
              placement="bottom"
            >
              <el-form-item label="acme.tls.altPort" prop="acme.tls.altPort">
                <el-input
                  v-model.number="dataForm.acme.tls.altPort"
                  clearable
                />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="tlsType === 'acme' && dataForm.acme.type === 'dns'"
              :content="$t('hysteria.config.acme.dns.name')"
              placement="bottom"
            >
              <el-form-item label="acme.dns.name" prop="acme.dns.name">
                <el-select v-model="dataForm.acme.dns.name" style="width: 100%">
                  <el-option
                    v-for="item in dnsNames"
                    :key="item"
                    :label="item"
                    :value="item"
                  />
                </el-select>
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="tlsType === 'acme' && dataForm.acme.type === 'dns'"
              :content="$t('hysteria.config.acme.dns.config')"
              placement="bottom"
            >
              <el-form-item label="acme.dns.config" prop="acme.dns.config">
                <dns-credentials v-model="dataForm.acme.dns.config" :provider="dataForm.acme.dns.name" />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="tlsType === 'acme' && !dataForm.acme.type"
              :content="$t('hysteria.config.acme.disableHTTP')"
              placement="bottom"
            >
              <el-form-item label="acme.disableHTTP" prop="acme.disableHTTP">
                <el-switch v-model="dataForm.acme.disableHTTP" />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="tlsType === 'acme' && !dataForm.acme.type"
              :content="$t('hysteria.config.acme.disableTLSALPN')"
              placement="bottom"
            >
              <el-form-item
                label="acme.disableTLSALPN"
                prop="acme.disableTLSALPN"
              >
                <el-switch v-model="dataForm.acme.disableTLSALPN" />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="tlsType === 'acme' && !dataForm.acme.type"
              :content="$t('hysteria.config.acme.altHTTPPort')"
              placement="bottom"
            >
              <el-form-item label="acme.altHTTPPort" prop="acme.altHTTPPort">
                <el-input
                  v-model.number="dataForm.acme.altHTTPPort"
                  clearable
                />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="tlsType === 'acme' && !dataForm.acme.type"
              :content="$t('hysteria.config.acme.altTLSALPNPort')"
              placement="bottom"
            >
              <el-form-item
                label="acme.altTLSALPNPort"
                prop="acme.altTLSALPNPort"
              >
                <el-input
                  v-model.number="dataForm.acme.altTLSALPNPort"
                  clearable
                />
              </el-form-item>
            </el-tooltip>
          </el-tab-pane>
          <el-tab-pane :label="$t('hysteria.extension')" name="extension">
            <el-tooltip
              :content="$t('hysteria.config.remark')"
              placement="bottom"
            >
              <el-form-item label="remark" prop="remark">
                <el-input v-model="configForm.remark" clearable />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              :content="$t('hysteria.config.portHopping')"
              placement="bottom"
            >
              <el-form-item label="portHopping" prop="portHopping">
                <el-input v-model="configForm.portHopping" clearable />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              :content="$t('hysteria.config.clashExtension')"
              placement="bottom"
            >
              <el-form-item label="clashExtension" prop="clashExtension">
                <el-input
                  v-model="configForm.clashExtension"
                  type="textarea"
                  :autosize="{ minRows: 3 }"
                  @keydown="(e:KeyboardEvent) => e.stopPropagation()"
                />
              </el-form-item>
            </el-tooltip>
          </el-tab-pane>
          <el-tab-pane :label="$t('hysteria.listen')" name="listen">
            <el-tooltip
              :content="$t('hysteria.config.listen')"
              placement="bottom"
            >
              <el-form-item label="listen" prop="listen">
                <el-input v-model="dataForm.listen" clearable />
              </el-form-item>
            </el-tooltip>
          </el-tab-pane>
          <el-tab-pane :label="$t('hysteria.ech')" name="ech">
            <el-tooltip
              :content="$t('hysteria.config.ech.keyPath')"
              placement="bottom"
            >
              <el-form-item label="ech.keyPath" prop="ech.keyPath">
                <el-input v-model="dataForm.ech.keyPath" clearable />
              </el-form-item>
            </el-tooltip>
          </el-tab-pane>
          <el-tab-pane :label="$t('hysteria.obfs')" name="obfs">
            <el-tooltip
              :content="$t('hysteria.config.obfs.type')"
              placement="bottom"
            >
              <el-form-item label="obfs.type" prop="obfs.type">
                <el-select
                  v-model="dataForm.obfs.type"
                  style="width: 100%"
                  clearable
                >
                  <el-option
                    v-for="item in obfsTypes"
                    :key="item"
                    :label="item"
                    :value="item"
                  />
                </el-select>
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="dataForm.obfs.type === 'salamander'"
              :content="$t('hysteria.config.obfs.salamander.password')"
              placement="bottom"
            >
              <el-form-item
                label="obfs.salamander.password"
                prop="obfs.salamander.password"
              >
                <el-input
                  v-model="dataForm.obfs.salamander.password"
                  type="password"
                  show-password
                  clearable
                />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="dataForm.obfs.type === 'gecko'"
              :content="$t('hysteria.config.obfs.gecko.password')"
              placement="bottom"
            >
              <el-form-item
                label="obfs.gecko.password"
                prop="obfs.gecko.password"
              >
                <el-input
                  v-model="dataForm.obfs.gecko.password"
                  type="password"
                  show-password
                  clearable
                />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="dataForm.obfs.type === 'gecko'"
              :content="$t('hysteria.config.obfs.gecko.minPacketSize')"
              placement="bottom"
            >
              <el-form-item
                label="obfs.gecko.minPacketSize"
                prop="obfs.gecko.minPacketSize"
              >
                <el-input
                  v-model.number="dataForm.obfs.gecko.minPacketSize"
                  clearable
                />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="dataForm.obfs.type === 'gecko'"
              :content="$t('hysteria.config.obfs.gecko.maxPacketSize')"
              placement="bottom"
            >
              <el-form-item
                label="obfs.gecko.maxPacketSize"
                prop="obfs.gecko.maxPacketSize"
              >
                <el-input
                  v-model.number="dataForm.obfs.gecko.maxPacketSize"
                  clearable
                />
              </el-form-item>
            </el-tooltip>
          </el-tab-pane>
          <el-tab-pane :label="$t('hysteria.quic')" name="quic">
            <el-tooltip
              :content="$t('hysteria.config.quic.initStreamReceiveWindow')"
              placement="bottom"
            >
              <el-form-item
                label="quic.initStreamReceiveWindow"
                prop="quic.initStreamReceiveWindow"
              >
                <el-input
                  v-model.number="dataForm.quic.initStreamReceiveWindow"
                  clearable
                />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              :content="$t('hysteria.config.quic.maxStreamReceiveWindow')"
              placement="bottom"
            >
              <el-form-item
                label="quic.maxStreamReceiveWindow"
                prop="quic.maxStreamReceiveWindow"
              >
                <el-input
                  v-model.number="dataForm.quic.maxStreamReceiveWindow"
                  clearable
                />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              :content="$t('hysteria.config.quic.initConnReceiveWindow')"
              placement="bottom"
            >
              <el-form-item
                label="quic.initConnReceiveWindow"
                prop="quic.initConnReceiveWindow"
              >
                <el-input
                  v-model.number="dataForm.quic.initConnReceiveWindow"
                  clearable
                />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              :content="$t('hysteria.config.quic.maxConnReceiveWindow')"
              placement="bottom"
            >
              <el-form-item
                label="quic.maxConnReceiveWindow"
                prop="quic.maxConnReceiveWindow"
              >
                <el-input
                  v-model.number="dataForm.quic.maxConnReceiveWindow"
                  clearable
                />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              :content="$t('hysteria.config.quic.maxIdleTimeout')"
              placement="bottom"
            >
              <el-form-item
                label="quic.maxIdleTimeout"
                prop="quic.maxIdleTimeout"
              >
                <el-input v-model="dataForm.quic.maxIdleTimeout" clearable />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              :content="$t('hysteria.config.quic.maxIncomingStreams')"
              placement="bottom"
            >
              <el-form-item
                label="quic.maxIncomingStreams"
                prop="quic.maxIncomingStreams"
              >
                <el-input
                  v-model.number="dataForm.quic.maxIncomingStreams"
                  clearable
                />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              :content="$t('hysteria.config.quic.disablePathMTUDiscovery')"
              placement="bottom"
            >
              <el-form-item
                label="quic.disablePathMTUDiscovery"
                prop="quic.disablePathMTUDiscovery"
              >
                <el-switch v-model="dataForm.quic.disablePathMTUDiscovery" />
              </el-form-item>
            </el-tooltip>
          </el-tab-pane>
          <el-tab-pane
            :label="$t('hysteria.bandwidth')"
            name="bandwidth"
          >
            <el-tooltip
              :content="$t('hysteria.config.bandwidth.up')"
              placement="bottom"
              :trigger-keys="[]"
            >
              <el-form-item label="bandwidth.up" prop="bandwidth.up">
                <el-input v-model="dataForm.bandwidth.up" clearable />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              :content="$t('hysteria.config.bandwidth.down')"
              placement="bottom"
              :trigger-keys="[]"
            >
              <el-form-item label="bandwidth.down" prop="bandwidth.down">
                <el-input v-model="dataForm.bandwidth.down" clearable />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              :content="$t('hysteria.config.bandwidth.disableLossCompensation')"
              placement="bottom"
            >
              <el-form-item
                label="bandwidth.disableLossCompensation"
                prop="bandwidth.disableLossCompensation"
              >
                <el-switch
                  v-model="dataForm.bandwidth.disableLossCompensation"
                />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              :content="$t('hysteria.config.ignoreClientBandwidth')"
              placement="bottom"
            >
              <el-form-item
                label="ignoreClientBandwidth"
                prop="ignoreClientBandwidth"
              >
                <el-switch v-model="dataForm.ignoreClientBandwidth" />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              :content="$t('hysteria.config.congestion.type')"
              placement="bottom"
            >
              <el-form-item label="congestion.type" prop="congestion.type">
                <el-select
                  v-model="dataForm.congestion.type"
                  style="width: 100%"
                >
                  <el-option
                    v-for="item in congestionTypes"
                    :key="item"
                    :label="item"
                    :value="item"
                  />
                </el-select>
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="dataForm.congestion.type === 'bbr'"
              :content="$t('hysteria.config.congestion.bbrProfile')"
              placement="bottom"
            >
              <el-form-item
                label="congestion.bbrProfile"
                prop="congestion.bbrProfile"
              >
                <el-select
                  v-model="dataForm.congestion.bbrProfile"
                  style="width: 100%"
                >
                  <el-option
                    v-for="item in bbrProfiles"
                    :key="item"
                    :label="item"
                    :value="item"
                  />
                </el-select>
              </el-form-item>
            </el-tooltip>
          </el-tab-pane>
          <el-tab-pane
            :label="$t('hysteria.speedTest')"
            name="speedTest"
          >
            <el-tooltip
              :content="$t('hysteria.config.speedTest')"
              placement="bottom"
            >
              <el-form-item label="speedTest" prop="speedTest">
                <el-switch v-model="dataForm.speedTest" />
              </el-form-item>
            </el-tooltip>
          </el-tab-pane>
          <el-tab-pane :label="$t('hysteria.udp')" name="udp">
            <el-tooltip
              :content="$t('hysteria.config.disableUDP')"
              placement="bottom"
            >
              <el-form-item label="disableUDP" prop="disableUDP">
                <el-switch v-model="dataForm.disableUDP" />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              :content="$t('hysteria.config.udpIdleTimeout')"
              placement="bottom"
            >
              <el-form-item label="udpIdleTimeout" prop="udpIdleTimeout">
                <el-input v-model="dataForm.udpIdleTimeout" clearable />
              </el-form-item>
            </el-tooltip>
          </el-tab-pane>
          <el-tab-pane
            :label="$t('hysteria.resolver')"
            name="resolver"
          >
            <el-tooltip
              :content="$t('hysteria.config.resolver.type')"
              placement="bottom"
            >
              <el-form-item label="resolver.type" prop="resolver.type">
                <el-select v-model="dataForm.resolver.type" style="width: 100%">
                  <el-option
                    v-for="item in resolverTypes"
                    :key="item"
                    :label="item"
                    :value="item"
                  />
                </el-select>
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="dataForm.resolver.type === 'tcp'"
              :content="$t('hysteria.config.resolver.tcp.addr')"
              placement="bottom"
            >
              <el-form-item label="resolver.tcp.addr" prop="resolver.tcp.addr">
                <el-input v-model="dataForm.resolver.tcp.addr" clearable />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="dataForm.resolver.type === 'tcp'"
              :content="$t('hysteria.config.resolver.tcp.timeout')"
              placement="bottom"
            >
              <el-form-item
                label="resolver.tcp.timeout"
                prop="resolver.tcp.timeout"
              >
                <el-input v-model="dataForm.resolver.tcp.timeout" clearable />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="dataForm.resolver.type === 'udp'"
              :content="$t('hysteria.config.resolver.udp.addr')"
              placement="bottom"
            >
              <el-form-item label="resolver.udp.addr" prop="resolver.udp.addr">
                <el-input v-model="dataForm.resolver.udp.addr" clearable />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="dataForm.resolver.type === 'udp'"
              :content="$t('hysteria.config.resolver.udp.timeout')"
              placement="bottom"
            >
              <el-form-item
                label="resolver.udp.timeout"
                prop="resolver.udp.timeout"
              >
                <el-input v-model="dataForm.resolver.udp.timeout" clearable />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="dataForm.resolver.type === 'tls'"
              :content="$t('hysteria.config.resolver.tls.addr')"
              placement="bottom"
            >
              <el-form-item label="resolver.tls.addr" prop="resolver.tls.addr">
                <el-input v-model="dataForm.resolver.tls.addr" clearable />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="dataForm.resolver.type === 'tls'"
              :content="$t('hysteria.config.resolver.tls.timeout')"
              placement="bottom"
            >
              <el-form-item
                label="resolver.tls.timeout"
                prop="resolver.tls.timeout"
              >
                <el-input v-model="dataForm.resolver.tls.timeout" clearable />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="dataForm.resolver.type === 'tls'"
              :content="$t('hysteria.config.resolver.tls.sni')"
              placement="bottom"
            >
              <el-form-item label="resolver.tls.sni" prop="resolver.tls.sni">
                <el-input v-model="dataForm.resolver.tls.sni" clearable />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="dataForm.resolver.type === 'tls'"
              :content="$t('hysteria.config.resolver.tls.insecure')"
              placement="bottom"
            >
              <el-form-item
                label="resolver.tls.insecure"
                prop="resolver.tls.insecure"
              >
                <el-switch v-model="dataForm.resolver.tls.insecure" />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="dataForm.resolver.type === 'https'"
              :content="$t('hysteria.config.resolver.https.addr')"
              placement="bottom"
            >
              <el-form-item
                label="resolver.https.addr"
                prop="resolver.https.addr"
              >
                <el-input v-model="dataForm.resolver.https.addr" clearable />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="dataForm.resolver.type === 'https'"
              :content="$t('hysteria.config.resolver.https.timeout')"
              placement="bottom"
            >
              <el-form-item
                label="resolver.https.timeout"
                prop="resolver.https.timeout"
              >
                <el-input v-model="dataForm.resolver.https.timeout" clearable />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="dataForm.resolver.type === 'https'"
              :content="$t('hysteria.config.resolver.https.sni')"
              placement="bottom"
            >
              <el-form-item
                label="resolver.https.sni"
                prop="resolver.https.sni"
              >
                <el-input v-model="dataForm.resolver.https.sni" clearable />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="dataForm.resolver.type === 'https'"
              :content="$t('hysteria.config.resolver.https.insecure')"
              placement="bottom"
            >
              <el-form-item
                label="resolver.https.insecure"
                prop="resolver.https.insecure"
              >
                <el-switch v-model="dataForm.resolver.https.insecure" />
              </el-form-item>
            </el-tooltip>
          </el-tab-pane>
          <el-tab-pane :label="$t('hysteria.sniff')" name="sniff">
            <el-tooltip
              :content="$t('hysteria.config.sniff.enable')"
              placement="bottom"
            >
              <el-form-item label="sniff.enable" prop="sniff.enable">
                <el-switch v-model="dataForm.sniff.enable" />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              :content="$t('hysteria.config.sniff.timeout')"
              placement="bottom"
            >
              <el-form-item label="sniff.timeout" prop="sniff.timeout">
                <el-input v-model="dataForm.sniff.timeout" clearable />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              :content="$t('hysteria.config.sniff.rewriteDomain')"
              placement="bottom"
            >
              <el-form-item
                label="sniff.rewriteDomain"
                prop="sniff.rewriteDomain"
              >
                <el-switch v-model="dataForm.sniff.rewriteDomain" />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              :content="$t('hysteria.config.sniff.tcpPorts')"
              placement="bottom"
            >
              <el-form-item label="sniff.tcpPorts" prop="sniff.tcpPorts">
                <el-input v-model="dataForm.sniff.tcpPorts" clearable />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              :content="$t('hysteria.config.sniff.udpPorts')"
              placement="bottom"
            >
              <el-form-item label="sniff.udpPorts" prop="sniff.udpPorts">
                <el-input v-model="dataForm.sniff.udpPorts" clearable />
              </el-form-item>
            </el-tooltip>
          </el-tab-pane>
          <el-tab-pane :label="$t('hysteria.acl')" name="acl">
            <el-tooltip
              :content="$t('hysteria.config.aclType')"
              placement="bottom"
            >
              <el-form-item label="acl.type" prop="acl.type">
                <el-select v-model="aclType" style="width: 100%" clearable>
                  <el-option
                    v-for="item in aclTypes"
                    :key="item"
                    :label="item"
                    :value="item"
                  />
                </el-select>
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="aclType === 'file'"
              :content="$t('hysteria.config.acl.file')"
              placement="bottom"
            >
              <el-form-item label="acl.file" prop="acl.file">
                <el-input v-model="dataForm.acl.file" clearable />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="aclType === 'inline'"
              :content="$t('hysteria.config.acl.inline')"
              placement="bottom"
            >
              <el-form-item label="acl.inline" prop="acl.inline">
                <imputMultiple :tags="dataForm.acl.inline" />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              :content="$t('hysteria.config.acl.geoip')"
              placement="bottom"
            >
              <el-form-item label="acl.geoip" prop="acl.geoip">
                <el-input v-model="dataForm.acl.geoip" clearable />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              :content="$t('hysteria.config.acl.geosite')"
              placement="bottom"
            >
              <el-form-item label="acl.geosite" prop="acl.geosite">
                <el-input v-model="dataForm.acl.geosite" clearable />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              :content="$t('hysteria.config.acl.geoUpdateInterval')"
              placement="bottom"
            >
              <el-form-item
                label="acl.geoUpdateInterval"
                prop="acl.geoUpdateInterval"
              >
                <el-input v-model="dataForm.acl.geoUpdateInterval" clearable />
              </el-form-item>
            </el-tooltip>
          </el-tab-pane>
          <el-tab-pane
            :label="$t('hysteria.outbounds')"
            name="outbounds"
          >
            <Outbounds :outbounds="dataForm.outbounds" />
          </el-tab-pane>
          <el-tab-pane :label="$t('hysteria.http')" name="http">
            <el-tooltip
              :content="$t('hysteria.config.trafficStats.listen')"
              placement="bottom"
            >
              <el-form-item
                label="trafficStats.listen"
                prop="trafficStats.listen"
              >
                <el-input v-model="dataForm.trafficStats.listen" clearable />
              </el-form-item>
            </el-tooltip>
          </el-tab-pane>
          <el-tab-pane
            :label="$t('hysteria.masquerade')"
            name="masquerade"
          >
            <el-tooltip
              :content="$t('hysteria.config.masquerade.type')"
              placement="bottom"
            >
              <el-form-item label="masquerade.type" prop="masquerade.type">
                <el-select
                  v-model="dataForm.masquerade.type"
                  style="width: 100%"
                  clearable
                >
                  <el-option
                    v-for="item in masqueradeTypes"
                    :key="item"
                    :label="item"
                    :value="item"
                  />
                </el-select>
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="dataForm.masquerade.type === 'file'"
              :content="$t('hysteria.config.masquerade.file.dir')"
              placement="bottom"
            >
              <el-form-item
                label="masquerade.file.dir"
                prop="masquerade.file.dir"
              >
                <el-input v-model="dataForm.masquerade.file.dir" clearable />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="dataForm.masquerade.type === 'proxy'"
              :content="$t('hysteria.config.masquerade.proxy.url')"
              placement="bottom"
            >
              <el-form-item
                label="masquerade.proxy.url"
                prop="masquerade.proxy.url"
              >
                <el-input v-model="dataForm.masquerade.proxy.url" clearable />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="dataForm.masquerade.type === 'proxy'"
              :content="$t('hysteria.config.masquerade.proxy.rewriteHost')"
              placement="bottom"
            >
              <el-form-item
                label="masquerade.proxy.rewriteHost"
                prop="masquerade.proxy.rewriteHost"
              >
                <el-switch v-model="dataForm.masquerade.proxy.rewriteHost" />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="dataForm.masquerade.type === 'proxy'"
              :content="$t('hysteria.config.masquerade.proxy.insecure')"
              placement="bottom"
            >
              <el-form-item
                label="masquerade.proxy.insecure"
                prop="masquerade.proxy.insecure"
              >
                <el-switch v-model="dataForm.masquerade.proxy.insecure" />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="dataForm.masquerade.type === 'proxy'"
              :content="$t('hysteria.config.masquerade.proxy.xForwarded')"
              placement="bottom"
            >
              <el-form-item
                label="masquerade.proxy.xForwarded"
                prop="masquerade.proxy.xForwarded"
              >
                <el-switch v-model="dataForm.masquerade.proxy.xForwarded" />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="dataForm.masquerade.type === 'string'"
              :content="$t('hysteria.config.masquerade.string.content')"
              placement="bottom"
            >
              <el-form-item
                label="masquerade.string.content"
                prop="masquerade.string.content"
              >
                <el-input
                  v-model="dataForm.masquerade.string.content"
                  clearable
                />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="dataForm.masquerade.type === 'string'"
              :content="$t('hysteria.config.masquerade.string.headers')"
              placement="bottom"
            >
              <el-form-item
                label="masquerade.string.headers"
                prop="masquerade.string.headers"
              >
                <map-add :map-object="dataForm.masquerade.string.headers" />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              v-if="dataForm.masquerade.type === 'string'"
              :content="$t('hysteria.config.masquerade.string.statusCode')"
              placement="bottom"
            >
              <el-form-item
                label="masquerade.string.statusCode"
                prop="masquerade.string.statusCode"
              >
                <el-switch v-model="dataForm.masquerade.string.statusCode" />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              :content="$t('hysteria.config.masquerade.listenHTTP')"
              placement="bottom"
            >
              <el-form-item
                label="masquerade.listenHTTP"
                prop="masquerade.listenHTTP"
              >
                <el-input v-model="dataForm.masquerade.listenHTTP" clearable />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              :content="$t('hysteria.config.masquerade.listenHTTPS')"
              placement="bottom"
            >
              <el-form-item
                label="masquerade.listenHTTPS"
                prop="masquerade.listenHTTPS"
              >
                <el-input v-model="dataForm.masquerade.listenHTTPS" clearable />
              </el-form-item>
            </el-tooltip>
            <el-tooltip
              :content="$t('hysteria.config.masquerade.forceHTTPS')"
              placement="bottom"
            >
              <el-form-item
                label="masquerade.forceHTTPS"
                prop="masquerade.forceHTTPS"
              >
                <el-switch v-model="dataForm.masquerade.forceHTTPS" />
              </el-form-item>
            </el-tooltip>
          </el-tab-pane>
          </el-tabs>
        </div>
      </el-form>
    </el-card>
  </div>
</template>

<script lang="ts">
export default {
  name: "index",
};
</script>

<script setup lang="ts">
import DnsCredentials from "./dns-credentials.vue";
import {
  ConfigsUpdateDto,
  defaultHysteria2ServerConfig,
  Hysteria2ServerConfig,
} from "@/api/config/types";
import Outbounds from "./components/Outbounds/index.vue";
import { Select } from "@element-plus/icons-vue";
import {
  buildHysteria2ConfigPayload,
  Hysteria2OptionalFeatures,
} from "./config-form";
import {
  exportHysteria2ConfigApi,
  getHysteria2ConfigApi,
  importHysteria2ConfigApi,
  listConfigApi,
  updateConfigsApi,
  updateHysteria2ConfigApi,
  uploadCertFileApi,
} from "@/api/config";
import { useI18n } from "vue-i18n";
import { assignWith, deepCopy } from "@/utils/copy";
import {
  UploadFile,
  UploadRawFile,
  UploadRequestOptions,
} from "element-plus/lib/components";
import {
  hysteria2ChangeVersionApi,
  hysteria2RoutingStatusApi,
  listReleaseApi,
  restartHysteria2Api,
  startHysteria2Api,
  stopHysteria2Api,
} from "@/api/hysteria2";
import { Hysteria2RoutingStatus } from "@/api/hysteria2/types";
import { monitorHysteria2Api } from "@/api/monitor";
import { UploadUserFile } from "element-plus";

const { t } = useI18n();

const hysteria2Remark = "HYSTERIA2_CONFIG_REMARK";
const hysteria2ConfigPortHopping = "HYSTERIA2_CONFIG_PORT_HOPPING";
const clashExtension = "CLASH_EXTENSION";
const dataFormRef = ref(ElForm);

const dataFormRules = {
  listen: [
    {
      required: true,
      message: "Required",
      trigger: ["change", "blur"],
    },
  ],
  "trafficStats.listen": [
    {
      required: true,
      message: "Required",
      trigger: ["change", "blur"],
    },
  ],
};

const tlsTypes = ref<string[]>(["tls", "acme"]);
const congestionTypes = ref<string[]>(["bbr", "reno"]);
const bbrProfiles = ref<string[]>(["standard", "conservative", "aggressive"]);
const tlsSniGuards = ref<string[]>(["dns-san", "disable", "strict"]);
const aclTypes = ref<string[]>(["file", "inline"]);
const acmeCas = ref<string[]>(["zerossl", "letsencrypt"]);
const acmeTypes = ref<string[]>(["http", "tls", "dns"]);
const dnsNames = ref<string[]>([
  "cloudflare",
  "duckdns",
  "gandi",
  "godaddy",
  "namedotcom",
  "vultr",
]);
const obfsTypes = ref<string[]>(["salamander", "gecko"]);
const resolverTypes = ref<string[]>(["tcp", "udp", "tls", "https"]);
const masqueradeTypes = ref<string[]>(["file", "proxy", "string"]);

const state = reactive({
  configForm: {
    remark: "",
    portHopping: "",
    clashExtension: "",
  },
  dataForm: { ...defaultHysteria2ServerConfig } as Hysteria2ServerConfig,
  activeName: "tls",
  expandedGroups: ["basic", "tls", "authTraffic", "performance"],
  tlsType: "acme",
  aclType: "inline",
  ech: false,
  obfs: false,
  quic: false,
  bandwidth: false,
  speedTest: false,
  udp: false,
  resolver: false,
  sniff: false,
  acl: false,
  outbounds: false,
  masquerade: false,
  fileList: [] as UploadFile[],
  hysteria2Version: "",
  hysteria2Versions: [] as string[],
  hysteria2Monitor: {
    version: "",
    running: false,
  },
  crtFileList: [] as UploadUserFile[],
  keyFileList: [] as UploadUserFile[],
  routingStatus: {
    enabled: false,
    ready: false,
    activeUsers: 0,
    boundUsers: 0,
    unboundUsers: [],
    errors: [],
    kernelVersion: "-",
    kernelCompatible: false,
  } as Hysteria2RoutingStatus,
  lifecycleLoading: false,
});

const {
  configForm,
  activeName,
  expandedGroups,
  dataForm,
  tlsType,
  aclType,
  ech,
  obfs,
  quic,
  bandwidth,
  speedTest,
  udp,
  resolver,
  sniff,
  acl,
  outbounds,
  masquerade,
  fileList,
  hysteria2Version,
  hysteria2Versions,
  hysteria2Monitor,
  crtFileList,
  keyFileList,
  routingStatus,
  lifecycleLoading,
} = toRefs(state);

const tlsMissing = computed(() => {
  if (state.tlsType === "tls") {
    return state.dataForm.tls?.cert?.trim() && state.dataForm.tls?.key?.trim()
      ? "" : t("hysteria.tlsFilesMissing");
  }
  const acme = state.dataForm.acme;
  if (!acme?.domains?.some((domain) => domain.trim())) return t("hysteria.acmeDomainsMissing");
  if (acme.type === "dns" && (!acme.dns?.name || !Object.keys(acme.dns.config ?? {}).length)) {
    return t("hysteria.acmeDNSMissing");
  }
  return "";
});

const routingStatusDescription = computed(() => {
  const parts = [
    `${t("account.boundUsers")}: ${state.routingStatus.boundUsers}/${state.routingStatus.activeUsers}`,
    `${t("account.kernelVersion")}: ${state.routingStatus.kernelVersion}`,
  ];
  if (state.routingStatus.unboundUsers.length) {
    parts.push(
      `${t("account.unboundUsers")}: ${state.routingStatus.unboundUsers.join(", ")}`
    );
  }
  if (state.routingStatus.errors.length) {
    parts.push(state.routingStatus.errors.join("; "));
  }
  return parts.join(" · ");
});

const setRoutingStatus = async () => {
  const { data } = await hysteria2RoutingStatusApi();
  state.routingStatus = data;
};

const optionalFeatureState = (): Hysteria2OptionalFeatures => ({
  ech: state.ech,
  obfs: state.obfs,
  quic: state.quic,
  bandwidth: state.bandwidth,
  speedTest: state.speedTest,
  udp: state.udp,
  resolver: state.resolver,
  sniff: state.sniff,
  acl: state.acl,
  outbounds: state.outbounds,
  masquerade: state.masquerade,
});

const configNavClass = (name: string) => [
  "config-nav-item",
  { "is-active": state.activeName === name },
];

const handleOptionalToggle = (
  name: string,
  enabled: string | number | boolean
) => {
  if (enabled) {
    state.activeName = name;
  } else if (state.activeName === name) {
    state.activeName = "listen";
  }
};

const handleImport = (params: UploadRequestOptions) => {
  if (state.fileList.length > 0) {
    let formData = new FormData();
    formData.append("file", params.file);
    importHysteria2ConfigApi(formData).then(() => {
      ElMessage.success(t("common.success"));
    });
    state.fileList = [];
  }
};

const beforeImport = (file: UploadRawFile) => {
  if (!file.name.endsWith(".yaml")) {
    ElMessage.error("file format not supported");
    return false;
  }
  if (file.size / 1024 / 1024 > 2) {
    ElMessage.error("the file is too big, less than 2 MB");
    return false;
  }
};

const handleExport = async () => {
  let response = await exportHysteria2ConfigApi();
  try {
    const blob = new Blob([response.data], {
      type: "application/octet-stream",
    });
    let url = window.URL.createObjectURL(blob);
    let a = document.createElement("a");
    document.body.appendChild(a);
    a.href = url;
    let dis = response.headers["content-disposition"];
    a.download = dis.split("attachment; filename=")[1];
    a.click();
    window.URL.revokeObjectURL(url);
    ElMessage.success(t("common.success"));
  } catch (e) {
    /* empty */
  }
};

const handleReset = () => {
  state.configForm = {
    remark: "",
    portHopping: "",
    clashExtension: "",
  };
  state.dataForm = deepCopy(defaultHysteria2ServerConfig);
  state.activeName = "tls";
  state.tlsType = "acme";
  state.aclType = "inline";
  state.ech = false;
  state.obfs = false;
  state.quic = false;
  state.bandwidth = false;
  state.speedTest = false;
  state.udp = false;
  state.resolver = false;
  state.sniff = false;
  state.acl = false;
  state.outbounds = false;
  state.masquerade = false;
  state.fileList = [];
};

const submitForm = () => {
  dataFormRef.value.validate(async (valid: any) => {
    if (valid) {
      let configs: ConfigsUpdateDto[] = [
        {
          key: hysteria2Remark,
          value: state.configForm.remark,
        },
        {
          key: hysteria2ConfigPortHopping,
          value: state.configForm.portHopping,
        },
        {
          key: clashExtension,
          value: state.configForm.clashExtension,
        },
      ];
      const params = buildHysteria2ConfigPayload(
        state.dataForm,
        optionalFeatureState(),
        state.tlsType,
        state.aclType
      );
      try {
        await updateConfigsApi({ configUpdateDtos: configs });
        await updateHysteria2ConfigApi(params);
        ElMessage.success(t("common.success"));
        setConfig();
        setRoutingStatus();
      } catch {
        setConfig();
        setRoutingStatus();
      }
    }
  });
};

const setConfig = () => {
  listConfigApi({
    keys: [
      hysteria2Remark,
      hysteria2ConfigPortHopping,
      clashExtension,
    ],
  }).then((response) => {
    const data = response.data;
    data.forEach((configVo) => {
      if (configVo.key === hysteria2Remark) {
        state.configForm.remark = configVo.value;
      } else if (configVo.key === hysteria2ConfigPortHopping) {
        state.configForm.portHopping = configVo.value;
      } else if (configVo.key === clashExtension) {
        state.configForm.clashExtension = configVo.value;
      }
    });
  });

  getHysteria2ConfigApi().then((response) => {
    const data = response.data;
    if (data) {
      state.tlsType = data?.tls ? "tls" : "acme";
      state.aclType = data?.acl?.inline ? "inline" : "file";

      state.ech = !!data?.ech;
      state.obfs = !!data?.obfs;
      state.quic = !!data?.quic;
      state.bandwidth = !!data?.bandwidth;
      state.speedTest = !!data?.speedTest;
      state.udp = !!data?.disableUDP || !!data?.udpIdleTimeout;
      state.resolver = !!data?.resolver;
      state.sniff = !!data?.sniff;
      state.acl = !!data?.acl;
      state.outbounds = !!data?.outbounds;
      state.masquerade = !!data?.masquerade;

      state.dataForm = deepCopy(defaultHysteria2ServerConfig);
      assignWith(state.dataForm, data);
    }
  });
};

const setHysteria2Versions = async () => {
  const { data } = await listReleaseApi();
  state.hysteria2Versions = data;
};

const setHysteria2Monitor = async () => {
  const { data } = await monitorHysteria2Api();
  Object.assign(state.hysteria2Monitor, data);
};

const handleLifecycleAction = async (
  action: "start" | "stop" | "restart"
) => {
  if (state.lifecycleLoading) return;
  if (action !== "start") {
    await ElMessageBox.confirm(
      t(
        action === "restart"
          ? "hysteria.restartConfirm"
          : "hysteria.stopConfirm"
      ),
      t("common.confirm")
    );
  }
  state.lifecycleLoading = true;
  let completed = false;
  try {
    if (action === "start") await startHysteria2Api();
    else if (action === "stop") await stopHysteria2Api();
    else await restartHysteria2Api();
    completed = true;
  } finally {
    try {
      await setHysteria2Monitor();
    } finally {
      state.lifecycleLoading = false;
    }
  }
  if (completed) ElMessage.success(t("common.success"));
};

const handleHysteria2ChangeVersion = async () => {
  if (!state.hysteria2Version) {
    ElMessage.error("version number is required");
    return;
  }
  ElMessage.info(t("common.wait"));
  await hysteria2ChangeVersionApi({ version: state.hysteria2Version });
  ElMessage.success(t("common.success"));
  await setHysteria2Monitor();
};

const uploadCertFile = async (params: UploadRequestOptions) => {
  try {
    if (!state.dataForm.tls) {
      return;
    }
    if (
      !params.file.name.endsWith(".crt") &&
      !params.file.name.endsWith(".key")
    ) {
      ElMessage.error("file format not supported");
    }
    if (params.file.size > 1024 * 1024) {
      ElMessage.error("the file is too big");
    }
    let formData = new FormData();
    formData.append("file", params.file);
    const { data } = await uploadCertFileApi(formData);
    if (params.file.name.endsWith(".crt")) {
      state.dataForm.tls.cert = data;
    } else if (params.file.name.endsWith(".key")) {
      state.dataForm.tls.key = data;
    }
  } catch (e) {
    /* empty */
  }
};

onMounted(() => {
  setConfig();
  setHysteria2Versions();
  setHysteria2Monitor();
  setRoutingStatus();
});
</script>

<style lang="scss" scoped>
.el-card .el-form {
  max-width: 1280px;
  margin: 0 auto;
}

.config-workspace {
  display: grid;
  grid-template-columns: minmax(220px, 280px) minmax(0, 1fr);
  gap: 24px;
  align-items: start;
}

.lifecycle-toolbar {
  align-items: center;
}

.config-navigation {
  position: sticky;
  top: 16px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 8px;
  padding: 0 14px 12px;
}

.config-nav-item {
  display: flex;
  width: 100%;
  min-height: 38px;
  margin: 4px 0;
  padding: 7px 10px;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  color: var(--el-text-color-regular);
  background: transparent;
  border: 0;
  border-radius: 6px;
  cursor: pointer;
  text-align: left;
}

.config-nav-item:hover,
.config-nav-item.is-active {
  color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
}

.managed-config-note {
  margin: 8px 10px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
  line-height: 1.5;
}

.config-editor {
  min-width: 0;
}

.config-editor :deep(.el-tabs__header) {
  display: none;
}

@media (max-width: 768px) {
  .config-workspace {
    grid-template-columns: 1fr;
    gap: 16px;
  }

  .config-navigation {
    position: static;
  }
}
</style>
