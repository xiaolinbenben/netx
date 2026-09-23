<script setup lang="ts">
import { onMounted, ref } from "vue";
import { message } from "@/utils/message";
import { getSettings, saveSettings, type SettingGroup } from "@/api/settings";

defineOptions({
  name: "Settings"
});

const loading = ref(false);
const saving = ref(false);
const groups = ref<SettingGroup[]>([]);
const form = ref<Record<string, string>>({});

function errorText(error: unknown, fallback: string): string {
  const response = (error as { response?: { data?: { message?: string } } })
    ?.response;
  return response?.data?.message ?? fallback;
}

function placeholderOf(field: SettingGroup["fields"][number]): string {
  if (!field.secret) return field.placeholder;
  return field.configured
    ? `已配置（${field.hint}），留空表示不修改`
    : "未配置";
}

async function load() {
  loading.value = true;
  try {
    const { data } = await getSettings();
    groups.value = data?.groups ?? [];
    const values: Record<string, string> = {};
    groups.value.forEach(group => {
      group.fields.forEach(field => {
        // 密钥类字段不回显明文，留空表示不修改
        values[field.key] = field.secret ? "" : (field.value ?? "");
      });
    });
    form.value = values;
  } catch (error) {
    message(errorText(error, "加载系统配置失败"), { type: "error" });
  } finally {
    loading.value = false;
  }
}

async function save() {
  saving.value = true;
  const values: Record<string, string> = {};
  groups.value.forEach(group => {
    group.fields.forEach(field => {
      const value = form.value[field.key] ?? "";
      if (field.secret) {
        if (value !== "") values[field.key] = value;
        return;
      }
      values[field.key] = field.type === "bool" ? String(value) : value;
    });
  });
  try {
    await saveSettings(values);
    message("配置已保存", { type: "success" });
    await load();
  } catch (error) {
    message(errorText(error, "保存系统配置失败"), { type: "error" });
  } finally {
    saving.value = false;
  }
}

onMounted(load);
</script>

<template>
  <div v-loading="loading" class="main">
    <el-card
      v-for="group in groups"
      :key="group.key"
      class="mb-4!"
      shadow="never"
      :header="group.title"
    >
      <el-form label-position="top">
        <el-form-item
          v-for="field in group.fields"
          :key="field.key"
          :label="field.label"
        >
          <el-switch
            v-if="field.type === 'bool'"
            v-model="form[field.key]"
            active-value="true"
            inactive-value="false"
          />
          <el-input
            v-else
            v-model="form[field.key]"
            :type="field.type === 'textarea' ? 'textarea' : 'text'"
            :rows="4"
            :placeholder="placeholderOf(field)"
            clearable
          />
        </el-form-item>
      </el-form>
    </el-card>

    <el-card v-if="!loading && groups.length === 0" shadow="never">
      <span class="text-gray-500">暂无可配置项</span>
    </el-card>

    <div v-if="groups.length > 0" class="flex justify-end">
      <el-button type="primary" :loading="saving" @click="save">
        保存配置
      </el-button>
    </div>
  </div>
</template>
