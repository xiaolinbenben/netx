<script setup lang="ts">
import dayjs from "dayjs";
import { onMounted, reactive, ref } from "vue";
import { message } from "@/utils/message";
import { copyTextToClipboard } from "@pureadmin/utils";
import {
  generateCodes,
  listCodes,
  updateCodeStatus,
  type CodeStatus,
  type RedeemCode
} from "@/api/codes";

defineOptions({
  name: "Codes"
});

const statusOptions = [
  { label: "全部", value: "" },
  { label: "未使用", value: "unused" },
  { label: "已使用", value: "used" },
  { label: "已作废", value: "void" }
];

const statusMeta: Record<
  CodeStatus,
  { text: string; type: "success" | "info" | "danger" }
> = {
  unused: { text: "未使用", type: "success" },
  used: { text: "已使用", type: "info" },
  void: { text: "已作废", type: "danger" }
};

const loading = ref(false);
const submitting = ref(false);
const rows = ref<RedeemCode[]>([]);
const total = ref(0);
const query = reactive({
  page: 1,
  size: 20,
  status: "" as "" | CodeStatus,
  keyword: ""
});

const generateVisible = ref(false);
const generateForm = reactive({ note: "", plan: "极速版", subscriptionUrl: "" });

function errorText(error: unknown, fallback: string): string {
  const response = (error as { response?: { data?: { message?: string } } })
    ?.response;
  return response?.data?.message ?? fallback;
}

function formatTime(value: string): string {
  return value ? dayjs(value).format("YYYY-MM-DD HH:mm:ss") : "-";
}

async function fetchCodes() {
  loading.value = true;
  try {
    const { data } = await listCodes({
      page: query.page,
      size: query.size,
      status: query.status || undefined,
      keyword: query.keyword || undefined
    });
    rows.value = data?.items ?? [];
    total.value = data?.total ?? 0;
  } catch (error) {
    message(errorText(error, "加载兑换码失败"), { type: "error" });
  } finally {
    loading.value = false;
  }
}

function search() {
  query.page = 1;
  fetchCodes();
}

function reset() {
  query.status = "";
  query.keyword = "";
  search();
}

function openGenerate() {
  generateForm.note = "";
  generateForm.plan = "极速版";
  generateForm.subscriptionUrl = "";
  generateVisible.value = true;
}

async function submitGenerate() {
  submitting.value = true;
  try {
    await generateCodes({
      note: generateForm.note,
      plan: generateForm.plan,
      subscriptionUrl: generateForm.subscriptionUrl
    });
    generateVisible.value = false;
    message("兑换码已生成", { type: "success" });
    query.page = 1;
    await fetchCodes();
  } catch (error) {
    message(errorText(error, "生成兑换码失败"), { type: "error" });
  } finally {
    submitting.value = false;
  }
}

function copy(text: string, tip: string) {
  copyTextToClipboard(text)
    ? message(tip, { type: "success" })
    : message("复制失败", { type: "error" });
}

async function changeStatus(row: RedeemCode, status: CodeStatus) {
  try {
    await updateCodeStatus(row.id, status);
    message(status === "void" ? "已作废" : "已恢复", { type: "success" });
    await fetchCodes();
  } catch (error) {
    message(errorText(error, "操作失败"), { type: "error" });
  }
}

onMounted(fetchCodes);
</script>

<template>
  <div class="main">
    <el-card shadow="never">
      <el-form :inline="true" @submit.prevent>
        <el-form-item label="状态">
          <el-select
            v-model="query.status"
            class="w-[140px]!"
            @change="search"
          >
            <el-option
              v-for="item in statusOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="兑换码">
          <el-input
            v-model="query.keyword"
            class="w-[220px]!"
            clearable
            placeholder="输入兑换码或备注"
            @keyup.enter="search"
            @clear="search"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="search">查询</el-button>
          <el-button @click="reset">重置</el-button>
        </el-form-item>
      </el-form>

      <div class="mb-4 flex justify-end">
        <el-button type="primary" @click="openGenerate">生成兑换码</el-button>
      </div>

      <el-table v-loading="loading" :data="rows" border stripe>
        <el-table-column label="兑换码" min-width="220">
          <template #default="{ row }">
            <span class="font-mono">{{ row.code }}</span>
            <el-button
              link
              type="primary"
              class="ml-2!"
              @click="copy(row.code, '已复制兑换码')"
            >
              复制
            </el-button>
          </template>
        </el-table-column>
        <el-table-column prop="note" label="备注" min-width="160">
          <template #default="{ row }">{{ row.note || "-" }}</template>
        </el-table-column>
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-tag :type="statusMeta[row.status as CodeStatus].type">
              {{ statusMeta[row.status as CodeStatus].text }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="180">
          <template #default="{ row }">{{ formatTime(row.createdAt) }}</template>
        </el-table-column>
        <el-table-column label="使用时间" width="180">
          <template #default="{ row }">{{ formatTime(row.usedAt) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="100" fixed="right">
          <template #default="{ row }">
            <el-button
              v-if="row.status === 'unused'"
              link
              type="danger"
              @click="changeStatus(row, 'void')"
            >
              作废
            </el-button>
            <el-button
              v-else-if="row.status === 'void'"
              link
              type="primary"
              @click="changeStatus(row, 'unused')"
            >
              恢复
            </el-button>
            <span v-else class="text-gray-400">-</span>
          </template>
        </el-table-column>
      </el-table>

      <div class="mt-4 flex justify-end">
        <el-pagination
          v-model:current-page="query.page"
          v-model:page-size="query.size"
          :page-sizes="[10, 20, 50, 100]"
          :total="total"
          background
          layout="total, sizes, prev, pager, next"
          @size-change="search"
          @current-change="fetchCodes"
        />
      </div>
    </el-card>

    <el-dialog v-model="generateVisible" title="生成兑换码" width="420px">
      <el-form label-width="80px">
        <el-form-item label="套餐">
          <el-select v-model="generateForm.plan" style="width: 100%">
            <el-option label="极速版" value="极速版" />
            <el-option label="至尊版" value="至尊版" />
          </el-select>
        </el-form-item>
        <el-form-item label="3x-ui 订阅">
          <el-input v-model="generateForm.subscriptionUrl" placeholder="粘贴订阅链接" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input
            v-model="generateForm.note"
            maxlength="100"
            placeholder="可选，最多 100 字"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="generateVisible = false">取消</el-button>
        <el-button
          type="primary"
          :loading="submitting"
          @click="submitGenerate"
        >
          生成
        </el-button>
      </template>
    </el-dialog>

  </div>
</template>
