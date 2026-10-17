import type { Messages } from '..';

const settings: Messages['settings'] = {
  blocked: {
    title: 'Bloqueados e silenciados',
    blocked: 'Bloqueados',
    muted: 'Silenciados',
    empty: 'Ninguém por aqui.',
    unblock: 'Desbloquear',
    unmute: 'Reativar som',
    explainer: 'Pessoas bloqueadas não podem te ver nem falar com você em lugar nenhum. Pessoas silenciadas só somem dos seus feeds.',
  },
  contacts: {
    title: 'Contatos de confiança',
    intro: 'Quando você aperta SOS, essas pessoas recebem um SMS com um link ao vivo mostrando a última estação por onde você passou. Elas nunca veem suas conversas.',
    remove: 'Remover {name}',
    name: 'Nome',
    namePlaceholder: 'Mãe',
    mobile: 'Celular',
    add: 'Adicionar contato',
    failed: 'Falhou',
  },
  privacy: {
    title: 'Privacidade e seus dados',
    allowed: 'O que você permitiu',
    corePause: 'Desativar isso pausa a sua conta.',
  },
  consent: {
    core_service: {
      title: 'Manter minha conta',
      body: 'Guardar meu telefone (criptografado), minha persona anônima e o que eu publico para o app funcionar. Obrigatório.',
    },
    travel_matching: {
      title: 'Combinação de Travel Buddy',
      body: 'Usar minhas estações e horários de trajeto para sugerir pessoas na mesma rota. Só o trecho em comum aparece.',
    },
    ai_processing: {
      title: 'Verificações de segurança com IA',
      body: 'Deixar uma IA conferir minhas mensagens atrás de telefones ou @ escondidos antes de serem enviadas. Ela nunca age sozinha.',
    },
    analytics: {
      title: 'Estatísticas de uso anônimas',
      body: 'Contar quais recursos são usados, sem ligação comigo ou com a minha persona.',
    },
    marketing_push: {
      title: 'Novidades do app',
      body: 'Notificações de vez em quando sobre novos recursos e eventos na minha cidade.',
    },
  },
  profile: {
    avatar: 'Trocar avatar',
    avatarSaved: 'Avatar atualizado',
    title: 'Editar perfil',
    saved: 'Perfil salvo',
    becomeTitle: 'Virar {alias}?',
    becomeBody: 'Você pode trocar seu apelido uma vez a cada 30 dias.',
    change: 'Trocar',
    aliasFailed: 'Não foi possível trocar o apelido',
    newAlias: 'Trocar nome de usuário',
    aliasChanged: 'Nome de usuário trocado',
    aliasAvailable: 'Novo apelido disponível em {date}',
    bio: 'Bio',
    hereFor: 'Aqui para',
    stations: 'Estações (privadas)',
    nearHome: 'Perto de casa',
    nearWork: 'Perto do trabalho / faculdade',
    notSet: 'Não definido',
    vibes: 'Vibes ({count})',
  },
  reveal: {
    title: 'Cartão de revelação',
    saved: 'Salvo em privado',
    intro:
      'Isso fica criptografado e oculto. Numa conversa a dois, cada um de vocês pode revelar um passo de cada vez: primeiro o nome, depois a foto, depois o contato. Nada aparece até vocês dois concordarem, e você pode voltar atrás quando quiser.',
    firstName: 'Primeiro nome',
    firstNamePlaceholder: 'Só o primeiro nome',
    photo: 'Foto',
    changePhoto: 'Trocar foto',
    addPhoto: 'Adicionar foto',
    contact: 'Contato',
    contactPlaceholder: 'Telefone ou um @ de rede social',
  },
};

export default settings;
