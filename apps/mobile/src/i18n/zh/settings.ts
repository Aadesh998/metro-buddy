import type { Messages } from '..';

const settings: Messages['settings'] = {
  blocked: {
    title: '已拉黑和已静音',
    blocked: '已拉黑',
    muted: '已静音',
    empty: '这里空空的。',
    unblock: '取消拉黑',
    unmute: '取消静音',
    explainer: '被拉黑的人在任何地方都看不到你，也联系不到你。被静音的人只是从你的动态里消失。',
  },
  contacts: {
    title: '紧急联系人',
    intro: '当你按下 SOS 时，这些人会收到一条短信，里面有实时链接，显示你最后经过的站点。他们看不到你的聊天。',
    remove: '移除 {name}',
    name: '姓名',
    namePlaceholder: '姐姐',
    mobile: '手机号',
    add: '添加联系人',
    failed: '失败',
  },
  privacy: {
    title: '隐私与你的数据',
    allowed: '你已允许的内容',
    corePause: '关闭此项会暂停你的账号。',
  },
  consent: {
    core_service: {
      title: '运行我的账号',
      body: '保存我的手机号（加密）、匿名身份和我发布的内容，让应用正常运行。必选。',
    },
    travel_matching: {
      title: '旅伴匹配',
      body: '使用我的通勤站点和时间，推荐同一路线上的人。只会显示行程中重合的那一段。',
    },
    ai_processing: {
      title: 'AI 安全检查',
      body: '在消息发出前，让 AI 再检查一下是否藏有手机号或社交账号。它绝不会自行采取行动。',
    },
    analytics: {
      title: '匿名使用统计',
      body: '统计哪些功能被使用，不会关联到我或我的匿名身份。',
    },
    marketing_push: {
      title: '产品动态',
      body: '偶尔推送新功能和我所在城市的活动。',
    },
  },
  profile: {
    avatar: '更换头像',
    avatarSaved: '头像已更新',
    title: '编辑资料',
    saved: '资料已保存',
    becomeTitle: '改名为 {alias}？',
    becomeBody: '昵称每 30 天只能更改一次。',
    change: '更改',
    aliasFailed: '无法更改昵称',
    newAlias: '更改用户名',
    aliasChanged: '用户名已更改',
    aliasAvailable: '{date} 后可更改昵称',
    bio: '简介',
    hereFor: '来这里是为了',
    stations: '站点（私密）',
    nearHome: '家附近',
    nearWork: '公司 / 学校附近',
    notSet: '未设置',
    vibes: '气质（{count}）',
  },
  reveal: {
    title: '身份卡',
    saved: '已私密保存',
    intro:
      '这些信息会加密并隐藏。在一对一聊天中，你和对方可以各自选择一步步公开：先是名字，然后是照片，最后是联系方式。只有双方都同意才会显示，你也可以随时收回。',
    firstName: '名字',
    firstNamePlaceholder: '只填你的名字',
    photo: '照片',
    changePhoto: '更换照片',
    addPhoto: '添加照片',
    contact: '联系方式',
    contactPlaceholder: '手机号或社交账号',
  },
};

export default settings;
