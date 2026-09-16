
export interface PaginatedResponse<T> {
  /** 表示查询是否成功。 */
  success: boolean;
  /** 当前页数据。 */
  data: T[];
  /** 符合条件的总记录数。 */
  total: number;
  /** 当前页码。 */
  page: number;
  /** 每页记录数。 */
  page_size: number;
  /** 总页数。 */
  total_pages: number;
  /** 各触发类型的规则计数。 */
  trigger_counts?: Record<string, number>;
}

// Auth
/** 会话登录和首次初始化接口的具名响应。 */
export interface SessionResponse {
  /** 表示登录或初始化是否成功。 */
  success: boolean;
  /** 登录成功后的会话 Token。 */
  token?: string;
  /** 服务端返回的操作说明。 */
  message?: string;
  /** 当前用户 ID。 */
  user_id?: number;
  /** 当前用户名。 */
  username?: string;
  /** 当前用户是否为管理员。 */
  is_admin?: boolean;
}

/** 商品发布和发货地选择共用的只读地点契约。 */
export interface PublishLocation {
  /** 行政区划展示名称。 */
  area: string;
  /** 城市展示名称。 */
  city: string;
  /** 平台使用的行政区划标识。 */
  division_id: string;
  /** 地点经度。 */
  longitude: number;
  /** 地点纬度。 */
  latitude: number;
  /** 平台兴趣点标识。 */
  poi_id: string;
  /** 平台兴趣点展示名称。 */
  poi_name: string;
  /** 省份展示名称。 */
  province: string;
}

// Accounts
export interface AccountDetail {
  /** 闲鱼账号稳定标识。 */
  id: string;
  /** 账号是否已配置平台凭证；摘要接口只返回状态，不返回 Cookie 明文。 */
  cookie_configured?: boolean;
  /** 账号是否允许运行。 */
  enabled: boolean;
  /** 是否自动确认订单。 */
  auto_confirm: boolean;
  /** 用户为账号设置的备注。 */
  remark?: string;
  /** 自动回复暂停时长，单位为分钟。 */
  pause_duration?: number;
  /** 暂停结束时间的 Unix 秒。 */
  paused_until?: number;
  /** 当前是否处于暂停状态。 */
  paused?: boolean;
  // 登录信息
  /** 用于密码登录的闲鱼用户名。 */
  username?: string;
  /** 是否已保存密码登录秘密；摘要接口只返回状态，不返回密码明文。 */
  login_password_configured?: boolean;
  /** 是否在密码登录时显示浏览器。 */
  show_browser?: boolean;
  // Frontend helpers
  /** 平台账号昵称。 */
  nickname?: string;
  /** 平台账号头像地址。 */
  avatar_url?: string;
  /** 资料刷新失败时的说明。 */
  profile_error?: string;
  /** 当前账号运行状态。 */
  runtime_state?: 'starting' | 'connecting' | 'online' | 'reconnecting' | 'auth_expired' | 'verification_required' | 'runtime_conflict' | 'error' | 'stopped' | 'disabled';
  /** 当前运行状态的用户可见说明。 */
  runtime_message?: string;
  /** 当前运行实例是否已连接。 */
  runtime_connected?: boolean;
  /** 运行状态快照的更新时间。 */
  runtime_updated_at?: string;
  // AI设置
  /** 是否启用账号 AI 回复。 */
  ai_enabled?: boolean;
  /** 允许的最大折扣比例。 */
  max_discount_percent?: number;
  /** 允许的最大折扣金额。 */
  max_discount_amount?: number;
  /** 允许的最大砍价轮次。 */
  max_bargain_rounds?: number;
  /** 账号自定义提示词。 */
  custom_prompts?: string;
	// 账号级计划任务
	/** 是否启用自动评价。 */
	auto_rate_enabled?: boolean;
	/** 自动评价使用的文案。 */
	rate_content?: string;
	/** 是否启用每日擦亮。 */
	auto_polish_enabled?: boolean;
	/** 每日擦亮执行时间。 */
	polish_time?: string;
	/** 是否在付款延迟到期后自动向买家求花。 */
	auto_request_flower_enabled?: boolean;
	/** schema 37 旧客户端兼容的等待小时数。 */
	request_flower_after_hours?: number;
	/** 明确发货成功后等待多少秒再自动求花。 */
	request_flower_after_seconds?: number;
	/** 是否在收到买家送花卡片后自动收花。 */
	auto_receive_flower_enabled?: boolean;
	/** 自动收花时是否显示独立 Chromium。 */
	receive_flower_show_browser?: boolean;
	/** 等待平台已收花结果的最长秒数。 */
	receive_flower_timeout_seconds?: number;
	/** 是否启用发货后自动确认收货提醒。 */
	auto_receipt_reminder_enabled?: boolean;
	/** 发货后等待的完整天数。 */
	receipt_reminder_after_days?: number;
	/** 每日北京时间执行时刻。 */
	receipt_reminder_time?: string;
	/** 旧候选客户端兼容字段；官方系统卡片内容不可编辑。 */
	receipt_reminder_message?: string;
	/** 最近一次开启提醒的服务端基线 Unix 秒。 */
	receipt_reminder_enabled_at?: number;
	/** 最近一次自动评价扫描时间。 */
	last_rate_scan_at?: number;
	/** 最近一次擦亮日期。 */
	last_polish_date?: string;
	/** 最近一次擦亮时间。 */
	last_polish_at?: number;
}

export interface AccountTaskSettings {
	/** 账号稳定标识。 */
	account_id: string;
	/** 是否启用自动评价。 */
	auto_rate_enabled: boolean;
	/** 自动评价文案。 */
	rate_content: string;
	/** 是否启用每日擦亮。 */
	auto_polish_enabled: boolean;
	/** 每日擦亮执行时间。 */
	polish_time: string;
	/** 是否在付款延迟到期后自动向买家求花。 */
	auto_request_flower_enabled: boolean;
	/** schema 37 旧客户端兼容的等待小时数。 */
	request_flower_after_hours: number;
	/** 明确发货成功后等待多少秒再自动求花。 */
	request_flower_after_seconds: number;
	/** 是否在收到买家送花卡片后自动收花。 */
	auto_receive_flower_enabled: boolean;
	/** 自动收花时是否显示独立 Chromium。 */
	receive_flower_show_browser: boolean;
	/** 等待平台已收花结果的最长秒数。 */
	receive_flower_timeout_seconds: number;
	/** 是否启用发货后自动确认收货提醒。 */
	auto_receipt_reminder_enabled: boolean;
	/** 发货后等待的完整天数。 */
	receipt_reminder_after_days: number;
	/** 每日北京时间执行时刻。 */
	receipt_reminder_time: string;
	/** 旧候选客户端兼容字段；官方系统卡片内容不可编辑。 */
	receipt_reminder_message: string;
	/** 最近一次开启提醒的服务端基线 Unix 秒。 */
	receipt_reminder_enabled_at?: number;
	/** 最近一次自动评价扫描时间。 */
	last_rate_scan_at?: number;
	/** 最近一次擦亮日期。 */
	last_polish_date?: string;
	/** 最近一次擦亮时间。 */
	last_polish_at?: number;
}

export interface AccountTaskSummary {
	/** 任务类型。 */
	task_type: 'auto_rate' | 'auto_polish';
	/** 发现的任务数量。 */
	found: number;
	/** 成功处理的任务数量。 */
	success: number;
	/** 处理失败的任务数量。 */
	failed: number;
	/** 跳过的任务数量。 */
	skipped: number;
	/** 批量处理结果说明。 */
	message?: string;
}

export interface ChatSession {
	/** 账号稳定标识。 */
	account_id: string;
	/** 会话稳定标识。 */
	chat_id: string;
	/** 买家平台标识。 */
	buyer_id: string;
	/** 买家昵称。 */
	buyer_name: string;
	/** 买家头像地址。 */
	buyer_avatar_url?: string;
	/** 会话关联商品标识。 */
	item_id?: string;
	/** 会话关联商品标题。 */
	item_title?: string;
	/** 最近一条消息内容。 */
	last_message: string;
	/** 最近一条消息的 Unix 秒时间戳。 */
	last_message_at: number;
	/** 未读消息数量。 */
	unread_count: number;
	/** 是否已在所属 ERP 账号内持久化置顶；旧服务未返回时视为 false。 */
	is_pinned?: boolean;
}

/** ChatCreditLevel 是当前会话顶部展示的单个角色信用等级。 */
export interface ChatCreditLevel {
	/** role 区分买家和卖家信用。 */ role: 'buyer' | 'seller';
	/** level 是平台一至五级数字信用。 */ level: number;
	/** code 是平台稳定信用标签代码。 */ code: string;
	/** text 是平台当前中文等级文案。 */ text: string;
}

/** ChatCreditProfile 是按需查询当前买家的公开信用和缓存状态。 */
export interface ChatCreditProfile {
	/** user_id 是当前会话买家平台标识。 */ user_id: string;
	/** buyer 是买家信用；平台未返回时省略。 */ buyer?: ChatCreditLevel;
	/** seller 是卖家信用；平台未返回时省略。 */ seller?: ChatCreditLevel;
	/** fetched_at 是最近一次成功平台查询时间。 */ fetched_at: string;
	/** expires_at 是服务端缓存需要重新查询的时间。 */ expires_at: string;
	/** stale 表示当前因平台失败或熔断展示旧缓存。 */ stale: boolean;
}

/** 闲鱼 contentType=26 交易卡片的稳定事件。 */
export type ChatSystemCardEvent =
	| 'order_pending_payment'
	| 'order_price_adjusted'
	| 'order_paid'
	| 'order_shipped'
	| 'order_received'
	| 'order_completed'
	| 'order_closed'
	| 'order_cancelled'
	| 'refund_requested'
	| 'refund_completed'
	| 'red_flower_requested'
	| 'red_flower_prompted'
	| 'red_flower_sent'
	| 'red_flower_received'
	| 'unknown_trade_event';

/** 聊天页使用的非敏感只读交易卡片。 */
export interface ChatSystemCard {
	/** 卡片大类；第一阶段只支持交易卡片。 */
	kind: 'trade';
	/** 归一化后的交易状态。 */
	event: ChatSystemCardEvent;
	/** 平台交易卡片标题。 */
	title: string;
	/** 平台交易状态说明。 */
	description?: string;
	/** 从明确字段或动作地址解析出的订单标识。 */
	order_id?: string;
	/** 从明确字段或动作地址解析出的商品标识。 */
	item_id?: string;
	/** 已知卡片动作；收花动作只打开闲鱼官方页面，不由 ERP 调用安全 MTOP。 */
	action?: 'adjust_price' | 'ship_order' | 'receive_red_flower';
}

/** ChatLocationCard 是聊天页展示的自定义位置名称、说明和真实坐标。 */
export interface ChatLocationCard {
	/** 位置卡片主标题，例如实体店名称。 */ title: string;
	/** 地址、楼层或到店说明。 */ description: string;
	/** WGS84 纬度十进制度。 */ latitude: number;
	/** WGS84 经度十进制度。 */ longitude: number;
}

export interface ChatMessage {
	/** 消息数据库主键。 */
	id: number;
	/** 消息所属账号标识。 */
	account_id: string;
	/** 消息所属会话标识。 */
	chat_id: string;
	/** 平台消息去重键。 */
	message_key: string;
	/** 闲鱼撤回和历史对账使用的 PNM 平台消息标识。 */
	platform_message_id?: string;
	/** 当前消息原生引用的目标 PNM 平台消息标识。 */
	reply_to_platform_message_id?: string;
	/** 同账号同会话内解析的被引用消息展示快照。 */
	reply_preview?: ChatReplyPreview;
	/** 消息方向。 */
	direction: 'incoming' | 'outgoing';
	/** 发送者平台标识。 */
	sender_id: string;
	/** 发送者名称。 */
	sender_name: string;
  /** 消息类型，system 表示平台通知或交易卡片。 */
	message_type: 'text' | 'image' | 'video' | 'location' | 'system';
	/** 消息正文或媒体地址。 */
	content: string;
	/** 消息发送状态。 */
	status: 'received' | 'sending' | 'sent' | 'failed' | 'recall_pending' | 'recall_unknown' | 'recalled';
	/** 平台确认撤回的 Unix 毫秒时间；未撤回时省略。 */
	recalled_at?: number;
	/** 撤回操作者类型；0 发送者、1 群主、2 系统、3 安全撤回。 */
	recall_operator_type?: number;
	/** 平台撤回操作者标识。 */
	recall_operator_id?: string;
	/** contentType=26 的结构化只读交易卡片；普通系统通知省略。 */
	system_card?: ChatSystemCard;
	/** contentType=30 的结构化位置卡片；其他消息省略。 */
	location_card?: ChatLocationCard;
	/** 平台已读状态；旧消息可能没有该字段。 */
	read_status?: number;
	/** 平台确认已读的 Unix 秒时间戳；未确认时省略。 */
	read_at?: number;
	/** 消息发送时间的 Unix 秒。 */
	sent_at: number;
}

/** ChatReplyPreview 是聊天气泡展示原生引用关系所需的最小非敏感契约。 */
export interface ChatReplyPreview {
	/** 被引用消息的 PNM ID。 */ platform_message_id: string;
	/** 被引用消息相对当前账号的方向。 */ direction: 'incoming' | 'outgoing';
	/** 被引用消息发送者的平台标识。 */ sender_id: string;
	/** 被引用消息发送者的展示名称。 */ sender_name: string;
	/** 被引用消息的内容类型。 */ message_type: 'text' | 'image' | 'video' | 'location' | 'system';
	/** 被引用消息的原文或媒体地址。 */ content: string;
	/** 被引用消息的当前状态。 */ status: ChatMessage['status'];
}

// Orders
export type OrderStatus =
  | 'processing'
  | 'pending_ship'
  | 'shipped'
  | 'received'
  | 'completed'
  | 'cancelled'
  | 'refunding'
  | 'refunded'
  | 'unknown';

export interface Order {
  /** 订单本地数据库主键。 */
  id: string;
  /** 平台订单标识。 */
  order_id: string;
  /** 订单所属账号标识。 */
  cookie_id: string;
  /** 订单关联商品标识。 */
  item_id: string;
  /** 商品标题。 */
  item_title?: string;
  /** 商品图片地址。 */
  item_image?: string;
  /** 商品价格文本。 */
  item_price?: string;
  /** 买家平台标识。 */
  buyer_id: string;
	/** 订单精确关联的聊天会话标识；旧订单尚未关联时为空。 */
	chat_id?: string;
	/** 闲鱼买家昵称；本地尚未解析时省略。 */
	buyer_name?: string;
	/** 买家最近聊天会话提供的头像地址。 */
	buyer_avatar_url?: string;
  /** 购买数量。 */
  quantity: number;
  /** 订单金额文本。 */
  amount: string;
  /** 前端归一化后的订单状态。 */
  status: OrderStatus;
  /** 服务端原始订单状态。 */
  order_status?: OrderStatus;
  /** 收货人姓名。 */
  receiver_name?: string;
  /** 收货人电话。 */
  receiver_phone?: string;
  /** 收货地址。 */
  receiver_address?: string;
  /** 订单创建时间。 */
  created_at?: string;
  /** 订单更新时间。 */
  updated_at?: string;
	/** 平台明确付款的时间。 */
	paid_at?: string;
	/** 卖家明确发货的时间。 */
	shipped_at?: string;
	/** 买家明确确认收货的时间。 */
	received_at?: string;
	/** 平台明确交易完成的时间。 */
	completed_at?: string;
	/** 平台明确退款成功的时间。 */
	refunded_at?: string;
	/** 普通取消订单的时间。 */
	cancelled_at?: string;
	/** 是否存在 ERP 成功发货后保存的本地凭证。 */
	shipment_proof_available?: boolean;
	/** 凭证按钮禁用时的明确原因。 */
	shipment_proof_unavailable_reason?: string;
}

// Cards
export interface Card {
  /** 卡券组数值主键。 */
  id: number;
  /** 卡券组名称。 */
  name: string;
  /** 卡券组类型。 */
  type: 'api' | 'text' | 'data' | 'image';
  /** 卡券组说明。 */
  description?: string;
  /** 卡券组是否启用。 */
  enabled: boolean;
  // 文本类型
  text_content?: string;
  // 批量数据类型
  data_content?: string;
  // API 类型配置
  api_config?: {
    /** API 卡券请求地址。 */
    url: string;
    /** API 卡券请求方法。 */
    method: 'GET' | 'POST';
    /** API 请求超时时间。 */
    timeout?: number;
    /** API 请求头 JSON。 */
    headers?: string;
    /** API 请求参数 JSON。 */
    params?: string;
  };
  // 图片类型
  /** 图片卡券地址。 */
  image_url?: string;
  // 通用配置
  /** 卡券发送延迟秒数。 */
  delay_seconds?: number;
  // 多规格配置
  /** 是否支持多规格。 */
  is_multi_spec?: boolean;
  /** 规格名称。 */
  spec_name?: string;
  /** 规格值。 */
  spec_value?: string;
  /** 卡券组创建时间。 */
  created_at: string;
  /** 卡券组更新时间。 */
  updated_at: string;
}

// Items
/** 商品 SKU 的单个销售规格。 */
export interface ItemSKUProperty {
  /** 规格名称，例如“套餐”。 */
  name: string;
  /** 规格值，例如“一倍额度”。 */
  value: string;
  /** 规格值当前是否可选。 */
  enabled: boolean;
  /** 平台规格状态码。 */
  status: number;
  /** 规格值展示顺序。 */
  sort_order: number;
}

/** 卖家编辑详情同步的 SKU、实时库存和本地成本。 */
export interface ItemSKU {
  /** 平台 SKU 标识，字符串可避免长整型精度损失。 */
  sku_id: string;
  /** 平台库存标识，字符串可避免长整型精度损失。 */
  inventory_id: string;
  /** 组成当前 SKU 的销售规格。 */
  properties: ItemSKUProperty[];
  /** 平台售价，单位为人民币分。 */
  price_cents: number;
  /** 卖家编辑页返回的当前剩余库存。 */
  quantity: number;
  /** 发布时设置的初始库存。 */
  initial_quantity: number;
  /** SKU 当前是否在售可选。 */
  enabled: boolean;
  /** 平台 SKU 展示顺序。 */
  sort_order: number;
  /** 仅保存在 ERP 本地的单件成本分值；null 表示尚未填写。 */
  cost_cents: number | null;
  /** 库存快照同步时间，Unix 秒。 */
  synced_at: number;
  /** 是否为单规格商品仅在本地使用的隐式成本行。 */
  local_only: boolean;
}

export interface Item {
  /** 本地商品数据库主键。 */
  id: string | number;
  /** 商品所属账号标识。 */
  cookie_id: string;
  /** 平台商品标识。 */
  item_id: string;
  /** 商品标题。 */
  item_title?: string;
  /** 商品描述。 */
  item_description?: string;
  /** 商品价格文本。 */
  item_price?: string;
  /** 商品主图地址。 */
  item_image?: string; // Inferred from common usage, though not explicitly in list model sometimes
  /** 商品分类标识。 */
  item_category?: string;
  /** 商品详情原始 JSON。 */
  item_detail?: string;
  /** 是否启用多规格。 */
  is_multi_spec?: number | boolean;
  /** 是否按数量发货。 */
  multi_quantity_delivery?: number | boolean;
  /** 是否启用多数量发货兼容字段。 */
  is_multi_qty_ship?: number | boolean;
  /** 当前有效 SKU 数量。 */
  sku_count?: number;
  /** 当前有效 SKU、平台库存和本地成本。 */
  skus?: ItemSKU[];
  /** 商品创建时间。 */
  created_at?: string;
}

export type AutomationTriggerType = 'order_paid' | 'buyer_reviewed' | 'review_missing_timeout';
export type AutomationActionType = 'confirm_shipment' | 'send_card' | 'send_text';

// Rules
export interface ShippingRule {
  /** 规则标识。 */
  id: string;
  /** 规则名称。 */
  name: string;
  /** 规则触发类型。 */
  trigger_type: AutomationTriggerType;
  /** 规则匹配关键词。 */
  item_keyword: string; // Legacy UI helper
  /** 规则限定的账号标识。 */
  cookie_id?: string;
  /** 规则限定的商品标识。 */
  item_id?: string;
  /** 规则限定的商品标题。 */
  item_title?: string;
  /** 首个发卡动作使用的卡券组 ID。 */
  card_group_id: number; // First send_card action card id
  /** 首个发卡动作使用的卡券组名称。 */
  card_group_name?: string; // UI helper
  /** 规则优先级。 */
  priority: number;
  /** 规则是否启用。 */
  enabled: boolean;
  /** 规则原始配置 JSON。 */
  config_json?: string;
  /** 规则动作列表。 */
  actions: AutomationAction[];
  /** 规则规格变体列表。 */
  variants: ShippingVariant[];
}

export interface AutomationAction {
  /** 动作标识。 */
  id?: string;
  /** 动作类型。 */
  action_type: AutomationActionType;
  /** 动作使用的卡券组 ID。 */
  card_id?: number;
  /** 动作使用的卡券组名称。 */
  card_name?: string;
  /** 本次发放数量。 */
  delivery_count?: number;
  /** 文本消息模板。 */
  message_template?: string;
  /** 动作延迟秒数。 */
  delay_seconds?: number;
  /** 动作原始配置 JSON。 */
  config_json?: string;
  /** 动作是否启用。 */
  enabled: boolean;
  /** 动作排序序号。 */
  sort_order?: number;
}

export interface ShippingVariant {
  /** 规格变体标识。 */
  id?: string;
  /** 规格名称。 */
  spec_name: string;
  /** 规格值。 */
  spec_value: string;
  /** 变体使用的卡券组 ID。 */
  card_id: number;
  /** 变体使用的卡券组名称。 */
  card_name?: string;
  /** 卡券类型。 */
  card_type?: Card['type'];
  /** 变体发放数量。 */
  delivery_count: number;
  /** 变体是否启用。 */
  enabled: boolean;
  /** 是否覆盖动作级延迟。 */
  delay_override?: boolean;
  /** 变体延迟秒数。 */
  delay_seconds?: number;
  /** 变体原始配置 JSON。 */
  config_json?: string;
}

export interface ReplyRule {
  /** 回复规则标识。 */
  id: string;
  /** 触发关键词。 */
  keyword: string;
  /** 同一回复组内任意一个命中即可触发的关键词集合。 */
  keywords?: string[];
  /** 回复正文。 */
  reply_content: string;
  /** 关键词匹配方式。 */
  match_type: 'contains' | 'excludes' | 'equals' | 'exact' | 'fuzzy';
  /** 规则是否启用。 */
  enabled: boolean;
  /** 规则限定的商品标识。 */
  item_id?: string;
  /** 回复类型。 */
  type?: 'text' | 'image';
  /** 图片回复地址。 */
  image_url?: string;
  /** 规则处理客户消息或特定系统消息。 */
  message_scope?: 'customer' | 'system';
  /** 可同时选择客户消息和系统消息来源。 */
  message_scopes?: Array<'customer' | 'system'>;
  /** 系统消息规则允许处理的 contentType 白名单。 */
  system_types?: string[];
  /** 执行该全局规则组的闲鱼店铺账号集合。 */
  account_ids?: string[];
  /** 各绑定店铺独立的规则启用状态。 */
  account_states?: Array<{ /** 店铺账号标识。 */ account_id: string; /** 是否执行规则。 */ enabled: boolean }>;
  /** 同一会话重复回复冷却秒数。 */
  reply_interval_seconds?: number;
  /** 关键词命中后等待发送秒数。 */
  send_delay_seconds?: number;
}

// Stats
export interface AdminStats {
  /** 用户总数。 */
  total_users: number;
  /** 账号总数。 */
  total_cookies: number;
  /** 启用账号数。 */
  active_cookies: number;
  /** 卡券组总数。 */
  total_cards: number;
  /** 关键词规则总数。 */
  total_keywords: number;
  /** 订单总数。 */
  total_orders: number;
}

export interface DashboardStats {
  /** 账号总数。 */
  total_cookies: number;
  /** 启用账号数。 */
  active_cookies: number;
  /** 卡券组总数。 */
  total_cards: number;
  /** 关键词规则总数。 */
  total_keywords: number;
  /** 订单总数。 */
  total_orders: number;
  /** 可用卡券库存量。 */
  available_card_stock: number;
}

export interface OrderAnalytics {
  /** 收入汇总。 */
  revenue_stats: {
    /** 总收入金额。 */
    total_amount: number;
    /** 总订单数。 */
    total_orders: number;
  };
  /** 按日期聚合的订单统计。 */
  daily_stats: Array<{
    /** 统计日期。 */
    date: string;
    /** 当日订单金额。 */
    amount: number;
    /** 当日订单数量。 */
    order_count: number;
  }>;
  /** 按商品聚合的订单统计。 */
  item_stats?: Array<{
    /** 商品标识。 */
    item_id: string;
    /** 当前或历史本地记录解析出的商品标题。 */
    item_title?: string;
    /** 商品订单数。 */
    order_count: number;
    /** 商品订单总金额。 */
    total_amount: number;
    /** 商品订单平均金额。 */
    avg_amount: number;
  }>;
  /** 仅按精确成本快照计算的商品毛利汇总。 */
  profit_stats?: AnalyticsProfitStatsResponse;
  /** 按日统计的商品毛利。 */
  daily_profit_stats?: AnalyticsDailyProfitStatsResponse[];
  /** 按商品统计的商品毛利。 */
  item_profit_stats?: AnalyticsItemProfitStatsResponse[];
}

// Settings
export interface SystemSettings {
  /** 默认 AI 模型名称。 */
  ai_model?: string;
  /** 全局 AI API 密钥。 */
  ai_api_key?: string;
  /** 全局 AI API 密钥是否已在服务端配置。 */
  ai_api_key_configured?: boolean;
  /** 全局 AI API 地址。 */
  ai_api_url?: string;
  /** 全局 AI 基础地址。 */
  ai_base_url?: string;
  /** 系统默认回复文案。 */
  default_reply?: string;
  /** 是否允许注册新用户。 */
  registration_enabled?: boolean;
  /** 系统 SMTP 服务器地址。 */
  smtp_server?: string;
  /** 系统 SMTP 密码是否已在服务端配置。 */
  smtp_password_configured?: boolean;
  /** 服务端日志级别。 */
  log_level?: 'debug' | 'info' | 'warn' | 'error' | string;
  /** 服务端日志格式。 */
  log_format?: 'text' | 'json' | string;
  /** 续期日志保留天数。 */
  renewal_log_retention_days?: number;
  /** 远程验证码服务地址。 */
  'captcha.remote_service_url'?: string;
  /** 远程验证码服务密钥。 */
  'captcha.remote_secret_key'?: string;
  /** 远程验证码服务密钥是否已在服务端配置。 */
  'captcha.remote_secret_key_configured'?: boolean;
  /** 远程验证码服务 Cookie 配置。 */
  'captcha.remote_pass_cookies'?: boolean | string;
	/** 聊天位置卡片默认标题。 */
	'chat.location_card_title'?: string;
	/** 聊天位置卡片默认地址或到店说明。 */
	'chat.location_card_content'?: string;
	/** 聊天位置卡片默认纬度文本。 */
	'chat.location_card_latitude'?: string;
	/** 聊天位置卡片默认经度文本。 */
	'chat.location_card_longitude'?: string;
  /** 兼容未来配置键的扩展字段。 */
  [key: string]: any;
}

export interface AIReplySettings {
  /** 是否启用账号 AI 回复。 */
  ai_enabled: boolean;
  /** 账号使用的模型名称。 */
  model_name?: string;
  /** 账号 AI API 密钥。 */
  api_key?: string;
  /** 账号 AI 基础地址。 */
  base_url?: string;
  /** 最大折扣比例。 */
  max_discount_percent: number;
  /** 最大折扣金额。 */
  max_discount_amount?: number;
  /** 最大砍价轮次。 */
  max_bargain_rounds: number;
  /** 自定义提示词。 */
  custom_prompts: string;
}

// Default Reply
export interface DefaultReply {
  /** 账号稳定标识。 */
  cookie_id: string;
  /** 默认回复是否启用。 */
  enabled: boolean;
  /** 默认回复正文。 */
  reply_content: string;
  /** 是否对同一会话只回复一次。 */
  reply_once: boolean;
  /** 默认图片回复地址。 */
  reply_image_url?: string;
}

// 通知渠道
export type NotificationChannelType = 'dingtalk' | 'feishu' | 'bark' | 'webhook' | 'wechat' | 'telegram' | 'email';
export type NotificationEventType =
  | 'account_offline'
  | 'account_recovered'
  | 'account_disabled'
  | 'security_verification'
  | 'token_renewal'
  | 'delivery_result'
  | 'system_error';

export interface NotificationChannel {
  /** 通知渠道标识。 */
  id: string;
  /** 通知渠道名称。 */
  name: string;
  /** 通知渠道类型。 */
  type: NotificationChannelType;
  /** 通知渠道配置。 */
  config: Record<string, unknown>;
  /** 渠道绑定的事件类型。 */
  event_types?: NotificationEventType[];
  /** 通知渠道是否启用。 */
  enabled: boolean;
  /** 创建时间。 */
  created_at?: string;
  /** 更新时间。 */
  updated_at?: string;
}

/** 统一 HTTP 失败响应，客户端不再依赖 detail 或 msg 别名。 */
export interface ApiErrorResponse {
  /** 稳定的机器可读错误码。 */
  code: string;
  /** 可直接展示的错误说明。 */
  message: string;
  /** 可选的服务端请求追踪标识。 */
  request_id?: string;
  /** 仅供恢复或审计使用的结构化附加信息。 */
  details?: Record<string, unknown>;
}

/** 账号列表接口返回的非敏感具名 DTO。 */
export interface AccountSummaryResponse {
  /** 闲鱼账号稳定标识。 */
  id: string;
  /** 数据库中是否存在账号记录。 */
  has_cookie: boolean;
  /** 账号是否允许运行。 */
  enabled: boolean;
  /** 是否自动确认订单。 */
  auto_confirm: boolean;
  /** 账号备注。 */
  remark: string;
  /** 自动回复暂停时长，单位为分钟。 */
  pause_duration: number;
  /** 暂停结束 Unix 秒。 */
  paused_until: number;
  /** 当前是否仍处于暂停状态。 */
  paused: boolean;
  /** 密码登录用户名。 */
  username: string;
  /** 是否允许密码登录显示浏览器；兼容旧服务端的 0/1 字符串值。 */
  show_browser: boolean | number | string;
  /** 平台昵称缓存。 */
  nickname: string;
  /** 平台头像地址。 */
  avatar_url: string;
  /** 最近一次成功登录方式。 */
  login_method: string;
  /** 最近一次成功登录时间。 */
  last_login_at: number;
  /** 资料刷新错误说明。 */
  profile_error: string;
  /** 账号级 AI 回复开关。 */
  ai_enabled: boolean;
  /** 自动评价计划开关。 */
  auto_rate_enabled: boolean;
  /** 自动评价文案。 */
  rate_content: string;
  /** 自动擦亮计划开关。 */
  auto_polish_enabled: boolean;
  /** 自动擦亮本地时间。 */
  polish_time: string;
  /** 发货成功后是否自动向买家求花。 */
  auto_request_flower_enabled: boolean;
  /** 旧客户端兼容的发货后等待小时数。 */
  request_flower_after_hours: number;
  /** 发货成功后等待多少秒自动求花。 */
  request_flower_after_seconds: number;
  /** 收到买家送花卡片后是否自动收花。 */
  auto_receive_flower_enabled: boolean;
  /** 自动收花时是否显示独立浏览器窗口。 */
  receive_flower_show_browser: boolean;
  /** 等待平台收花结果的最长秒数。 */
  receive_flower_timeout_seconds: number;
	/** 是否启用发货后自动确认收货提醒。 */
	auto_receipt_reminder_enabled: boolean;
	/** 发货后等待的完整天数。 */
	receipt_reminder_after_days: number;
	/** 每日北京时间执行时刻。 */
	receipt_reminder_time: string;
	/** 旧候选客户端兼容字段；官方系统卡片内容不可编辑。 */
	receipt_reminder_message: string;
	/** 最近一次开启提醒的服务端基线 Unix 秒。 */
	receipt_reminder_enabled_at: number;
  /** 最近一次自动评价扫描时间。 */
  last_rate_scan_at: number;
  /** 最近一次自动擦亮日期。 */
  last_polish_date: string;
  /** 最近一次自动擦亮时间。 */
  last_polish_at: number;
}

/** 账号设置变更接口的具名成功响应。 */
export interface CookieSettingsResponse {
  /** 表示设置是否保存成功。 */
  success: boolean;
  /** 暂停结束 Unix 秒。 */
  paused_until: number;
  /** 表示账号当前是否暂停。 */
  paused: boolean;
}

/** 账号资料刷新接口的具名响应。 */
export interface CookieProfileResponse {
  /** 表示资料刷新是否成功。 */
  success: boolean;
  /** 账号稳定标识。 */
  id: string;
  /** 平台账号昵称。 */
  nickname: string;
  /** 平台账号头像地址。 */
  avatar_url: string;
  /** 资料刷新错误说明。 */
  profile_error: string;
}

/** 账号暂停时长查询接口的具名响应。 */
export interface PauseDurationResponse {
  /** 暂停时长，单位为分钟。 */
  pause_duration: number;
  /** 暂停结束 Unix 秒。 */
  paused_until: number;
  /** 表示账号当前是否暂停。 */
  paused: boolean;
}

/** 单个本地商品详情接口的具名响应。 */
export interface ItemDetailResponse {
  /** 商品所属账号标识。 */
  cookie_id: string;
  /** 平台商品标识。 */
  item_id: string;
  /** 商品标题。 */
  item_title: string;
  /** 商品描述。 */
  item_description: string;
  /** 商品分类标识。 */
  item_category: string;
  /** 商品价格文本。 */
  item_price: string;
  /** 商品详情原始 JSON。 */
  item_detail: string;
  /** 是否有多规格。 */
  is_multi_spec: boolean;
  /** 是否按数量发货。 */
  multi_quantity_delivery: boolean;
  /** 当前有效 SKU 数量。 */
  sku_count: number;
  /** 当前有效 SKU、平台库存和本地成本。 */
  skus: ItemSKU[];
}

/** 商品发布接口的具名成功响应。 */
export interface ItemPublishResponse {
  /** 表示商品是否发布成功。 */
  success: boolean;
  /** 发布结果说明。 */
  message: string;
  /** 新商品的平台标识。 */
  item_id: string;
  /** 新商品的平台详情地址。 */
  item_url: string;
  /** 新商品主图地址。 */
  item_image: string;
  /** 新商品标题。 */
  item_title: string;
  /** 新商品价格文本。 */
  item_price: string;
  /** 新商品库存数量。 */
  quantity: number;
  /** 新商品分类标识。 */
  category_id: string;
  /** 新商品分类名称。 */
  category_name: string;
}

/** 商品全集同步接口的具名响应。 */
export interface ItemSyncResponse {
  /** 表示同步是否完成。 */
  success: boolean;
  /** 同步结果说明。 */
  message: string;
  /** 平台返回的商品总数。 */
  total_count: number;
  /** 平台商品总页数。 */
  total_pages: number;
  /** 本地保存的商品数量。 */
  saved_count: number;
  /** 本地删除标记的商品数量。 */
  deleted_count: number;
}

/** 商品分页同步接口的具名响应。 */
export interface ItemPageSyncResponse {
  /** 表示同步是否完成。 */
  success: boolean;
  /** 同步结果说明。 */
  message: string;
  /** 当前同步页码。 */
  page_number: number;
  /** 当前同步页大小。 */
  page_size: number;
  /** 当前页商品数量。 */
  current_count: number;
  /** 本地保存的商品数量。 */
  saved_count: number;
}

/** 订单详情接口返回的原始具名订单 DTO。 */
export interface OrderDTOResponse {
  /** 平台订单标识。 */
  order_id: string;
  /** 关联商品标识。 */
  item_id: string;
  /** 关联商品标题。 */
  item_title: string;
  /** 关联商品图片地址。 */
  item_image: string;
  /** 买家平台标识。 */
  buyer_id: string;
  /** 商品规格名称。 */
  spec_name: string;
  /** 商品规格值。 */
  spec_value: string;
  /** 购买数量文本。 */
  quantity: string;
  /** 实付金额文本。 */
  amount: string;
  /** 归一化订单状态。 */
  order_status: string;
  /** 兼容前端使用的订单状态别名。 */
  status: string;
  /** 所属账号标识。 */
  cookie_id: string;
  /** 是否议价订单。 */
  is_bargain: number;
  /** 是否系统发货。 */
  system_shipped: boolean;
  /** 收货人姓名。 */
  receiver_name: string;
  /** 收货人电话。 */
  receiver_phone: string;
  /** 收货地址。 */
  receiver_address: string;
  /** 收货城市。 */
  receiver_city: string;
  /** 创建时间。 */
  created_at: string;
  /** 更新时间。 */
  updated_at: string;
}

/** 订单详情接口的具名响应。 */
export interface OrderDetailResponse extends OrderDTOResponse {
  /** 表示查询是否完成。 */
  success: boolean;
  /** 新版客户端读取的订单对象。 */
  data: OrderDTOResponse;
}

/** 订单单条刷新返回的远端详情。 */
export interface OrderRefreshDetailResponse {
  /** 购买数量文本。 */
  quantity: string;
  /** 商品规格名称。 */
  spec_name: string;
  /** 商品规格值。 */
  spec_value: string;
  /** 归一化订单状态。 */
  order_status: string;
  /** 实付金额文本。 */
  amount: string;
}

/** 订单单条刷新接口的具名响应。 */
export interface OrderSingleRefreshResponse {
  /** 表示刷新是否完成。 */
  success: boolean;
  /** 刷新结果说明。 */
  message: string;
  /** 刷新后的订单详情。 */
  order: OrderRefreshDetailResponse;
}

/** 自动化规则动作的原始具名 DTO。 */
export interface AutomationActionResponse {
  /** 动作稳定标识。 */
  id: number;
  /** 动作类型。 */
  action_type: string;
  /** 关联卡券组标识。 */
  card_id: number;
  /** 关联卡券组名称。 */
  card_name: string;
  /** 发送数量。 */
  delivery_count: number;
  /** 消息模板。 */
  message_template: string;
  /** 延迟秒数。 */
  delay_seconds: number;
  /** 扩展配置 JSON。 */
  config_json: string;
  /** 是否启用。 */
  enabled: boolean;
  /** 执行顺序。 */
  sort_order: number;
}

/** 自动化规则的原始具名 DTO。 */
export interface AutomationRuleResponse {
  /** 规则稳定标识。 */
  id: number;
  /** 所属账号标识。 */
  cookie_id: string;
  /** 关联商品标识。 */
  item_id: string;
  /** 关联商品标题。 */
  item_title: string;
  /** 规则名称。 */
  name: string;
  /** 触发类型。 */
  trigger_type: string;
  /** 是否启用。 */
  enabled: boolean;
  /** 规则优先级。 */
  priority: number;
  /** 扩展配置 JSON。 */
  config_json: string;
  /** 规则动作列表。 */
  actions: AutomationActionResponse[];
  /** 创建时间。 */
  created_at: string;
  /** 更新时间。 */
  updated_at: string;
}

/** 自动化规则分页接口的具名响应。 */
export interface AutomationRulePageResponse {
  /** 表示查询是否完成。 */
  success: boolean;
  /** 当前页规则列表。 */
  data: AutomationRuleResponse[];
  /** 规则总数。 */
  total: number;
  /** 当前页码。 */
  page: number;
  /** 当前页大小。 */
  page_size: number;
  /** 总页数。 */
  total_pages: number;
  /** 各触发类型规则数量。 */
  trigger_counts: Record<string, number>;
}

/** 订单批量变更接口的具名响应。 */
export interface OrderBatchResponse {
  /** 表示批量操作是否存在部分失败。 */
  partial_failure: boolean;
  /** 批量操作结果说明。 */
  message: string;
  /** 订单总数，导入接口提供。 */
  total?: number;
  /** 成功处理数量。 */
  success_count: number;
  /** 失败处理数量。 */
  failed_count: number;
	/** 逐订单兼容结果行。 */
	results: OrderBatchResult[];
}

/** 订单批量接口的逐订单结果行。 */
export interface OrderBatchResult {
  /** 订单平台标识。 */
  order_id?: string;
  /** 订单动作状态：failed、succeeded 或 reconciliation_required。 */
  status?: 'failed' | 'succeeded' | 'reconciliation_required';
  /** 表示该订单是否处理成功。 */
  success?: boolean;
  /** 该订单处理结果说明。 */
  message: string;
  /** 兼容接口可能返回的账号标识。 */
  cookie_id?: string;
  /** 待补偿记录标识。 */
  reconciliation_id?: string;
  /** 本地状态或补偿记录写入警告。 */
  reconciliation_warning?: string;
  /** 兼容接口可能返回的处理阶段。 */
  stage?: string;
  /** 允许后端保留尚未结构化的扩展字段。 */
  [key: string]: unknown;
}

/** 手动求花接口返回的平台动作状态。 */
export interface RedFlowerRequestResponse {
	/** success 表示平台明确接受了求花动作。 */
	success: boolean;
	/** status 区分成功、带凭证警告、失败和人工核对。 */
	status: 'not_requested' | 'running' | 'succeeded' | 'succeeded_with_warning' | 'failed' | 'needs_review';
	/** message 是不包含账号凭证的用户提示。 */
	message: string;
	/** requested_at 是求花动作首次开始的 Unix 秒时间戳。 */
	requested_at?: number;
}

/** 账号 AI 回复设置接口的具名响应。 */
export interface AIReplySettingsResponse {
  /** 账号稳定标识；默认配置响应可能省略。 */
  cookie_id?: string;
  /** AI 回复是否启用。 */
  ai_enabled: boolean;
  /** 最大折扣比例。 */
  max_discount_percent: number;
  /** 最大折扣金额。 */
  max_discount_amount: number;
  /** 最大砍价轮次。 */
  max_bargain_rounds: number;
  /** 自定义提示词。 */
  custom_prompts: string;
}

/** AI 模型发现接口的具名响应。 */
export interface AIModelsResponse {
  /** 远端可用模型名称。 */
  models: string[];
}

/** 单个用户设置查询接口的具名响应。 */
export interface UserSettingResponse {
  /** 设置值文本。 */
  value: string;
}

/** 卡券批量创建接口的逐行结果。 */
export interface CardBatchResult {
  /** 表格中的原始行号。 */
  row_no: number;
  /** 当前行是否创建成功。 */
  success: boolean;
  /** 新建卡券组主键。 */
  id?: number;
  /** 卡券组名称。 */
  name: string;
  /** 卡券类型。 */
  type?: string;
  /** 当前行失败原因。 */
  error?: string;
}

/** 卡券批量创建接口的具名响应。 */
export interface CardBatchResponse {
  /** 批量处理流程是否完成。 */
  success: boolean;
  /** 解析出的总行数。 */
  total: number;
  /** 创建成功行数。 */
  created: number;
  /** 创建失败行数。 */
  failed: number;
  /** 逐行处理结果。 */
  rows: CardBatchResult[];
}

/** 卡券追加数据接口的具名响应。 */
export interface CardAppendResponse {
  /** 追加操作是否完成。 */
  success: boolean;
  /** 实际追加数量。 */
  added: number;
}

/** 通知绑定列表中的单条记录。 */
export interface NotificationBinding {
  /** 账号稳定标识，列表归一化后补充。 */
  cookie_id?: string;
  /** 绑定记录主键。 */
  id?: number;
  /** 通知渠道主键。 */
  channel_id: number;
  /** 通知渠道名称。 */
  channel_name: string;
  /** 绑定是否启用。 */
  enabled: boolean;
}

/** 账号通知渠道绑定查询响应。 */
export interface AccountBindingsResponse {
  /** 账号稳定标识。 */
  cookie_id: string;
  /** 已绑定通知渠道主键列表。 */
  channel_ids: number[];
}

/** 商品类目推荐接口的具名响应。 */
export interface CategoryRecommendationResponse {
  /** 类目推荐是否成功。 */
  success: boolean;
  /** 推荐商品类目。 */
  category: {
    /** 平台类目主键。 */
    cat_id: string;
    /** 平台类目名称。 */
    cat_name: string;
    /** 频道类目主键。 */
    channel_cat_id: string;
    /** 淘宝类目主键。 */
    tb_cat_id?: string;
  };
}

/** 商品批量发布预检逐行结果。 */
export interface ItemPublishBatchPreviewRow {
  /** 上传表格行号。 */
  row_no: number;
  /** 当前行是否通过预检。 */
  valid: boolean;
  /** 当前行校验错误列表。 */
  errors?: string[];
  /** 发布目标账号标识。 */
  cookie_id: string;
  /** 商品标题。 */
  title: string;
  /** 商品价格文本。 */
  price: string;
  /** 商品库存数量。 */
  quantity: number;
  /** 商品图片引用列表。 */
  images: string[];
  /** 商品发布类目。 */
  category: CategoryRecommendationResponse['category'];
  /** 发布后自动化配置。 */
  automation?: Record<string, unknown>;
}

/** 商品批量发布预检响应。 */
export interface ItemPublishBatchPreviewResponse {
  /** 预检流程是否完成。 */
  success: boolean;
  /** 后续启动发布使用的预检批次标识。 */
  preview_id: string;
  /** 预检总行数。 */
  total: number;
  /** 通过预检行数。 */
  valid: number;
  /** 未通过预检行数。 */
  invalid: number;
  /** 逐行预检结果。 */
  rows: ItemPublishBatchPreviewRow[];
}

/** 商品批量发布任务启动或重试响应。 */
export interface BatchIDResponse {
  /** 任务操作是否完成。 */
  success: boolean;
  /** 商品批量任务标识。 */
  batch_id: string;
}

/** 商品批量发布任务取消响应。 */
export interface BatchCancelResponse {
  /** 取消请求是否完成。 */
  success: boolean;
  /** 取消后的任务状态。 */
  status: string;
}

/** 商品批量发布任务逐行详情。 */
export interface ItemPublishBatchRowResponse {
  /** 明细行主键。 */
  id: number;
  /** 导入表格行号。 */
  row_no: number;
  /** 发布目标账号标识。 */
  cookie_id: string;
  /** 商品标题。 */
  title: string;
  /** 商品价格文本。 */
  price: string;
  /** 商品库存数量。 */
  quantity: number;
  /** 商品图片引用列表。 */
  images: string[];
  /** 商品发布类目。 */
  category: CategoryRecommendationResponse['category'];
  /** 发布后自动化配置。 */
  automation: Record<string, unknown>;
  /** 明细行状态。 */
  status: string;
  /** 发布成功后的平台商品标识。 */
  item_id: string;
  /** 发布成功后的商品地址。 */
  item_url: string;
  /** 明细行失败原因。 */
  error_message: string;
  /** 明细行失败类型。 */
  failure_kind: string;
}

/** 商品批量发布任务详情响应。 */
export interface ItemPublishBatchResponse {
  /** 批量任务标识。 */
  id: string;
  /** 批量任务状态。 */
  status: string;
  /** 原始上传文件名。 */
  filename: string;
  /** 明细行总数。 */
  total: number;
  /** 成功发布数量。 */
  success: number;
  /** 失败数量。 */
  failed: number;
  /** 待处理数量。 */
  pending: number;
  /** 运行中数量。 */
  running: number;
  /** 可重试数量。 */
  retryable: number;
  /** 明细行结果。 */
  rows: ItemPublishBatchRowResponse[];
  /** 批次统一发货地。 */
  location?: Record<string, unknown>;
  /** 最终商品发布之间的最小间隔秒数。 */
  publish_interval_seconds?: number;
  /** 创建时间。 */
  created_at: string;
  /** 更新时间。 */
  updated_at: string;
}

/** 商品批量发布任务列表响应。 */
export interface ItemPublishBatchListResponse {
  /** 当前用户的批量任务列表。 */
  batches: ItemPublishBatchResponse[];
}

/** 简单资源创建接口的数值主键响应。 */
export interface MutationIDResponse {
  /** 资源创建是否完成。 */
  success: boolean;
  /** 新资源数值主键。 */
  id: number;
}

/** 简单变更接口的统一成功响应。 */
export interface OperationResponse {
  /** 操作是否完成。 */
  success: boolean;
  /** 可选的操作说明。 */
  message?: string;
  /** 操作完成后是否需要重新登录。 */
  requires_relogin?: boolean;
}

/** 通知渠道接口返回的原始具名 DTO。 */
export interface NotificationChannelResponse {
  /** 通知渠道主键。 */
  id: number;
  /** 通知渠道名称。 */
  name: string;
  /** 通知渠道类型。 */
  type: string;
  /** 通知渠道配置 JSON。 */
  config: string;
  /** 订阅事件类型 JSON 或兼容文本。 */
  event_types?: string;
  /** 通知渠道是否启用。 */
  enabled: boolean;
  /** 所属用户主键。 */
  user_id?: number;
  /** 创建时间；旧接口可能省略。 */
  created_at?: string;
  /** 更新时间；旧接口可能省略。 */
  updated_at?: string;
}

/** 卡券列表接口的兼容包装响应。 */
export interface CardListResponse {
  /** 当前用户卡券列表。 */
  cards: Card[];
}

/** 传统关键词列表项响应。 */
export interface KeywordBasicResponse {
  /** 匹配关键词。 */
  keyword: string;
  /** 文字回复内容。 */
  reply: string;
}

/** 带商品范围的关键词列表项响应。 */
export interface KeywordItemResponse extends KeywordBasicResponse {
  /** 限定的商品标识。 */
  item_id: string;
}

/** 带类型和主键的关键词列表项响应。 */
export interface KeywordTypedResponse extends KeywordItemResponse {
  /** 关键词规则主键。 */
  id: number;
  /** 回复类型。 */
  type: 'text' | 'image';
  /** 图片回复地址。 */
  image_url: string;
}

/** 指定商品回复项响应。 */
export interface ItemReplyResponse {
  /** 商品平台标识。 */
  item_id?: string;
  /** 账号稳定标识。 */
  cookie_id?: string;
  /** 指定商品的回复内容。 */
  reply_content: string;
}

/** 默认回复查询响应。 */
export interface DefaultReplyResponse extends DefaultReply {
  /** 账号稳定标识。 */
  cookie_id: string;
}

/** 账号任务设置响应。 */
export interface AccountTaskSettingsResponse {
  /** 账号稳定标识。 */
  account_id: string;
  /** 是否启用自动评价。 */
  auto_rate_enabled: boolean;
  /** 自动评价文案。 */
  rate_content: string;
  /** 是否启用自动擦亮。 */
  auto_polish_enabled: boolean;
  /** 自动擦亮本地时间。 */
  polish_time: string;
  /** 是否在付款延迟到期后自动向买家求花。 */
  auto_request_flower_enabled: boolean;
  /** schema 37 旧客户端兼容的等待小时数。 */
  request_flower_after_hours: number;
  /** 明确发货成功后等待多少秒再自动求花。 */
  request_flower_after_seconds: number;
  /** 是否在收到买家送花卡片后自动收花。 */
  auto_receive_flower_enabled: boolean;
  /** 自动收花时是否显示独立 Chromium。 */
  receive_flower_show_browser: boolean;
	/** 等待平台已收花结果的最长秒数。 */
	receive_flower_timeout_seconds: number;
	/** 是否启用发货后自动确认收货提醒。 */
	auto_receipt_reminder_enabled: boolean;
	/** 发货后等待的完整天数。 */
	receipt_reminder_after_days: number;
	/** 每日北京时间执行时刻。 */
	receipt_reminder_time: string;
	/** 旧候选客户端兼容字段；官方系统卡片内容不可编辑。 */
	receipt_reminder_message: string;
	/** 最近一次开启提醒的服务端基线 Unix 秒。 */
	receipt_reminder_enabled_at: number;
  /** 最近一次评价扫描时间。 */
  last_rate_scan_at: number;
  /** 最近一次擦亮日期。 */
  last_polish_date: string;
  /** 最近一次擦亮时间。 */
  last_polish_at: number;
}

/** 账号任务执行记录响应。 */
export interface AccountTaskRunResponse {
  /** 任务执行记录主键。 */
  id: number;
  /** 任务幂等键。 */
  run_key: string;
  /** 账号稳定标识。 */
  account_id: string;
  /** 任务类型。 */
  task_type: string;
  /** 任务目标标识。 */
  target_id: string;
  /** 任务业务日期。 */
  run_date: string;
  /** 任务执行状态。 */
  status: string;
  /** 任务成功数量。 */
  success_count: number;
  /** 任务失败数量。 */
  failed_count: number;
  /** 任务失败说明。 */
  error_message: string;
  /** 下一次重试时间。 */
  next_retry_at: number;
  /** 任务开始时间。 */
  started_at: number;
  /** 任务完成时间。 */
  finished_at: number;
}

/** 账号任务执行记录列表响应。 */
export interface AccountTaskRunsResponse {
  /** 当前账号的任务执行记录。 */
  runs: AccountTaskRunResponse[];
}

/** 手动执行账号任务的统计响应。 */
export interface AccountTaskSummaryResponse extends AccountTaskSummary {
  /** 任务结果说明。 */
  message?: string;
}

/** 手动执行账号任务的成功响应。 */
export interface AccountTaskRunResponseEnvelope {
  /** 任务请求是否成功完成。 */
  success: boolean;
  /** 账号任务执行统计。 */
  summary: AccountTaskSummaryResponse;
}

/** 管理员用户列表项响应。 */
export interface AdminUserResponse {
  /** 用户主键。 */
  id: number;
  /** 用户登录名。 */
  username: string;
  /** 用户邮箱。 */
  email: string;
  /** 用户是否启用。 */
  is_active: boolean;
  /** 用户是否为管理员。 */
  is_admin: boolean;
  /** 用户创建时间。 */
  created_at: string;
  /** 用户拥有的账号数量。 */
  cookie_count: number;
}

/** 管理员账号列表项响应。 */
export interface AdminCookieResponse {
  /** 账号稳定标识。 */
  id: string;
  /** 账号所属用户主键。 */
  user_id: number;
  /** 账号备注。 */
  remark: string;
  /** 账号创建时间。 */
  created_at: string;
  /** 账号所属用户名。 */
  owner: string;
  /** 账号是否启用。 */
  enabled: boolean;
}

/** 管理员全局统计响应。 */
export interface AdminStatsResponse extends AdminStats {}

/** 当前用户数据概览响应。 */
export interface DashboardStatsResponse extends DashboardStats {}

/** 订单收益统计响应。 */
export interface AnalyticsRevenueStatsResponse {
  /** 统计范围内的订单数。 */
  total_orders: number;
  /** 统计范围内的订单总金额。 */
  total_amount: number;
  /** 订单平均金额。 */
  avg_amount: number;
  /** 买家数量。 */
  unique_buyers: number;
  /** 商品数量。 */
  unique_items: number;
}

/** 按日期聚合的订单统计响应。 */
export interface AnalyticsDailyStatsResponse {
  /** 用户本地日期。 */
  date: string;
  /** 当天订单数。 */
  order_count: number;
  /** 当天订单金额。 */
  amount: number;
}

/** 按订单状态聚合的统计响应。 */
export interface AnalyticsStatusStatsResponse {
  /** 归一化后的订单状态。 */
  status: string;
  /** 该状态订单数。 */
  count: number;
  /** 该状态订单金额。 */
  amount: number;
}

/** 按收货城市聚合的统计响应。 */
export interface AnalyticsCityStatsResponse {
  /** 收货城市。 */
  city: string;
  /** 该城市订单数。 */
  order_count: number;
  /** 该城市订单金额。 */
  total_amount: number;
}

/** 按商品聚合的统计响应。 */
export interface AnalyticsItemStatsResponse {
  /** 商品平台标识。 */
  item_id: string;
  /** 当前或历史本地记录解析出的商品标题。 */
  item_title?: string;
  /** 该商品订单数。 */
  order_count: number;
  /** 该商品订单金额。 */
  total_amount: number;
  /** 该商品订单平均金额。 */
  avg_amount: number;
}

/** 仅基于精确成本快照计算的商品毛利汇总响应。 */
export interface AnalyticsProfitStatsResponse {
  /** 统计范围内的有效订单数。 */
  total_orders: number;
  /** 已精确匹配成本的订单数。 */
  covered_orders: number;
  /** 尚未精确匹配成本的订单数。 */
  unknown_cost_orders: number;
  /** 已匹配成本订单的成交金额。 */
  covered_revenue: number;
  /** 已匹配成本订单的商品成本。 */
  product_cost: number;
  /** 已覆盖订单按成交额 1.6% 逐单计算的平台手续费。 */
  platform_fee: number;
  /** 已匹配成本订单的预估商品毛利。 */
  gross_profit: number;
  /** 已完成或已收货订单的预估商品毛利。 */
  realized_gross_profit: number;
  /** 已覆盖成交额对应的毛利率百分比。 */
  gross_margin: number;
  /** 成本覆盖率百分比。 */
  coverage_rate: number;
}

/** 按日期聚合的预估商品毛利响应。 */
export interface AnalyticsDailyProfitStatsResponse {
  /** 用户本地日期。 */
  date: string;
  /** 当天有效订单数。 */
  total_orders: number;
  /** 当天已匹配成本订单数。 */
  covered_orders: number;
  /** 当天全部有效订单成交额。 */
  total_revenue: number;
  /** 当天已匹配成本订单成交额。 */
  covered_revenue: number;
  /** 当天已匹配成本订单商品成本。 */
  product_cost: number;
  /** 当天已覆盖订单的平台手续费。 */
  platform_fee: number;
  /** 当天已匹配成本订单预估商品毛利。 */
  gross_profit: number;
}

/** 按商品聚合的预估商品毛利响应。 */
export interface AnalyticsItemProfitStatsResponse {
  /** 商品平台标识。 */
  item_id: string;
  /** 商品标题。 */
  item_title: string;
  /** 该商品有效订单数。 */
  total_orders: number;
  /** 该商品已匹配成本订单数。 */
  covered_orders: number;
  /** 该商品已匹配成本订单成交额。 */
  covered_revenue: number;
  /** 该商品已匹配成本订单商品成本。 */
  product_cost: number;
  /** 该商品已覆盖订单的平台手续费。 */
  platform_fee: number;
  /** 该商品已匹配成本订单预估商品毛利。 */
  gross_profit: number;
  /** 已覆盖成交额对应的毛利率百分比。 */
  gross_margin: number;
  /** 该商品成本覆盖率百分比。 */
  coverage_rate: number;
}

/** 订单分析接口响应。 */
export interface OrderAnalyticsResponse {
  /** 收益统计。 */
  revenue_stats: AnalyticsRevenueStatsResponse;
  /** 按日统计。 */
  daily_stats: AnalyticsDailyStatsResponse[];
  /** 按状态统计。 */
  status_stats: AnalyticsStatusStatsResponse[];
  /** 按城市统计。 */
  city_stats: AnalyticsCityStatsResponse[];
  /** 按商品统计。 */
  item_stats: AnalyticsItemStatsResponse[];
  /** 仅按精确成本快照计算的商品毛利汇总。 */
  profit_stats?: AnalyticsProfitStatsResponse;
  /** 按日统计的商品毛利。 */
  daily_profit_stats?: AnalyticsDailyProfitStatsResponse[];
  /** 按商品统计的商品毛利。 */
  item_profit_stats?: AnalyticsItemProfitStatsResponse[];
}

/** 有效订单明细响应。 */
export interface ValidOrderResponse {
  /** 平台订单标识。 */
  order_id: string;
  /** 商品平台标识。 */
  item_id: string;
  /** 买家平台标识。 */
  buyer_id: string;
  /** 商品标题。 */
  item_title: string;
  /** 商品图片地址。 */
  item_image: string;
  /** 订单数量文本。 */
  quantity: string;
  /** 订单金额文本。 */
  amount: string;
  /** 兼容保留的订单状态。 */
  order_status: string;
  /** 归一化后的订单状态。 */
  status: string;
  /** 订单所属账号标识。 */
  cookie_id: string;
  /** 订单创建时间。 */
  created_at: string;
}

/** 有效订单分页响应。 */
export interface ValidOrdersResponse {
  /** 当前页有效订单。 */
  orders: ValidOrderResponse[];
  /** 符合条件的订单总数。 */
  total: number;
  /** 当前页码。 */
  page: number;
  /** 当前页大小。 */
  page_size: number;
  /** 是否还有未返回的订单。 */
  truncated: boolean;
}

/** 扫码登录二维码生成响应。 */
export interface QRLoginGenerateResponse {
  /** 二维码是否生成成功。 */
  success: boolean;
  /** 扫码登录会话标识。 */
  session_id: string;
  /** 二维码图片地址。 */
  qr_code_url: string;
  /** 可选的提示文本。 */
  message?: string;
}

/** 二维码登录状态响应。 */
export interface QRLoginStatusResponse {
  /** 当前二维码会话状态。 */
  status: string;
  /** 扫码登录会话标识。 */
  session_id?: string;
  /** 平台账号标识。 */
  unb?: string;
  /** 持久化后的本地账号标识。 */
  account_id?: string;
  /** 是否新建了本地账号。 */
  is_new_account?: boolean;
  /** 状态提示文本。 */
  message?: string;
  /** 兼容上游可能扩展的非敏感状态字段。 */
  [key: string]: unknown;
}

/** 二维码验证完成响应。 */
export interface QRLoginVerificationResponse {
  /** 验证结果是否成功。 */
  success: boolean;
  /** 平台账号标识。 */
  unb?: string;
  /** 持久化后的本地账号标识。 */
  account_id?: string;
  /** 是否新建了本地账号。 */
  is_new_account?: boolean;
  /** 扫码账号与目标账号不一致时的提示标识。 */
  scanned_account_id?: string;
  /** 验证结果提示文本。 */
  message?: string;
}

/** 订单列表刷新逐项结果。 */
export interface OrderRefreshResultResponse {
  /** 结果所属账号标识。 */
  cookie_id?: string;
  /** 当前处理阶段。 */
  stage?: string;
  /** 当前项是否处理成功。 */
  success: boolean;
  /** 结果说明。 */
  message?: string;
  /** 发现的新订单数量。 */
  discovered?: number;
  /** 更新的订单数量。 */
  updated?: number;
  /** 标记删除的订单数量。 */
  soft_deleted?: number;
	/** 安全跳过的跨账号归属冲突订单数量。 */
	conflict_skipped?: number;
  /** 订单平台标识。 */
  order_id?: string;
  /** 结果错误说明。 */
  error?: string;
}

/** 订单列表刷新统计摘要。 */
export interface OrderRefreshSummaryResponse {
	/** 本次同步尝试的账号数量。 */
	account_total?: number;
	/** 订单发现阶段成功的账号数量。 */
	account_succeeded?: number;
	/** 订单发现阶段失败的账号数量。 */
	account_failed?: number;
  /** 发现的新订单数量。 */
  discovered: number;
  /** 订单列表更新数量。 */
  list_updated: number;
  /** 标记删除数量。 */
  soft_deleted: number;
  /** 需要补全详情的订单数量。 */
  detail_total: number;
  /** 本次处理订单总数。 */
  total: number;
  /** 状态发生变化数量。 */
  updated: number;
  /** 状态未变化数量。 */
  no_change: number;
  /** 刷新失败数量。 */
  failed: number;
	/** 安全跳过的跨账号归属冲突订单数量。 */
	conflict_skipped?: number;
}

/** 订单列表刷新响应。 */
export interface OrderRefreshResponse {
  /** 是否存在部分失败。 */
  partial_failure: boolean;
  /** 刷新结果说明。 */
  message: string;
  /** 刷新统计摘要。 */
  summary: OrderRefreshSummaryResponse;
  /** 逐项兼容结果。 */
  results: OrderRefreshResultResponse[];
}

/** 创建订单刷新后台任务的响应。 */
export interface OrderRefreshJobStartResponse {
  /** 任务是否创建成功。 */
  success: boolean;
  /** 后台任务标识。 */
  job_id: string;
  /** 任务当前状态。 */
  status: 'queued' | 'running';
}

/** 订单刷新后台任务的轻量实时进度。 */
export interface OrderRefreshJobProgressResponse {
  /** 当前发现、订单导入、准备、详情、收尾或完成阶段。 */
  stage: 'discovering' | 'importing_orders' | 'preparing' | 'syncing_details' | 'finalizing' | 'completed';
  /** 当前阶段的用户可见说明。 */
  message: string;
  /** 当前阶段已经处理的账号或订单数量。 */
  processed: number;
  /** 当前阶段需要处理的账号或订单总数。 */
  total: number;
  /** 当前阶段成功数量。 */
  succeeded: number;
  /** 当前阶段失败数量。 */
  failed: number;
  /** 跨阶段整体进度，范围为 0 到 100。 */
  percent: number;
  /** 当前已完成的平台订单页码。 */
  current_page?: number;
  /** 当前账号平台订单总页数。 */
  total_pages?: number;
  /** 当前正在同步的账号序号，从 1 开始。 */
  current_account?: number;
  /** 本次任务需要同步的账号总数。 */
  total_accounts?: number;
	/** 当前同步范围；incremental 为默认增量，full 为完整校准。 */
	mode?: 'incremental' | 'full';
	/** 增量扫描已经连续命中的历史边界订单数。 */
	boundary_matched?: number;
	/** 增量扫描允许停止所需的连续历史订单数。 */
	boundary_required?: number;
}

/** 查询订单刷新后台任务的响应。 */
export interface OrderRefreshJobStatusResponse {
  /** 查询是否成功。 */
  success: boolean;
  /** 后台任务标识。 */
  job_id: string;
  /** 任务当前状态。 */
  status: 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled';
  /** 任务失败原因。 */
  error_message?: string;
  /** 运行中和完成任务的轻量实时进度。 */
  progress?: OrderRefreshJobProgressResponse;
  /** 任务成功后的订单刷新结果。 */
  result?: OrderRefreshResponse;
}

/** 取消订单刷新后台任务的响应。 */
export interface OrderRefreshJobCancelResponse {
  /** 取消命令是否成功应用。 */
  success: boolean;
  /** 被取消的任务标识。 */
  job_id: string;
  /** 取消后的任务状态。 */
  status: 'cancelled';
}

// ItemListEnvelope 是商品列表接口的兼容分页响应。
export interface ItemListEnvelope {
  /** items 是兼容分页响应中的商品列表。 */
  items?: Item[];
}

// AutomationIssuesEnvelope 是自动化异常接口的兼容响应。
export interface AutomationIssuesEnvelope {
  /** runs 是待处理的自动化运行记录。 */
  runs?: Array<{
    /** 记录标识。 */
    id: number;
    /** 所属账号标识。 */
    cookie_id: string;
    /** 所属订单标识。 */
    order_id: string;
    /** 自动化触发类型。 */
    trigger_type: string;
    /** 外部错误说明。 */
    error_message: string;
    /** 异常类别。 */
    issue_kind: 'external_result_unknown' | 'invalid_snapshot' | 'rule_unavailable' | 'partial_failure' | 'execution_failed';
    /** 允许的处理动作。 */
    allowed_resolutions: Array<'continue' | 'retry' | 'cancel'>;
    /** 当前动作游标。 */
    action_cursor: number;
    /** 已发送数量。 */
    sent_count: number;
    /** 更新时间。 */
    updated_at: string;
  }>;
  /** pending_tasks 是延迟自动化任务列表。 */
  pending_tasks?: Array<{
    /** 任务标识。 */
    id: number;
    /** 所属账号标识。 */
    cookie_id: string;
    /** 自动化触发类型。 */
    trigger_type: string;
    /** 错误说明。 */
    error_message: string;
    /** 当前重试次数。 */
    attempt_count: number;
    /** 更新时间。 */
    updated_at: string;
  }>;
}
