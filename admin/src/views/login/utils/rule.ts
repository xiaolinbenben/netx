import { reactive } from "vue";
import type { FormRules } from "element-plus";

/** 登录校验 */
const loginRules = reactive<FormRules>({
  password: [
    {
      required: true,
      message: "请输入密码",
      trigger: "blur"
    }
  ]
});

export { loginRules };
