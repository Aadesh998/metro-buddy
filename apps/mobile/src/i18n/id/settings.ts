import type { Messages } from '..';

const settings: Messages['settings'] = {
  blocked: {
    title: 'Diblokir & dibisukan',
    blocked: 'Diblokir',
    muted: 'Dibisukan',
    empty: 'Belum ada siapa-siapa.',
    unblock: 'Buka blokir',
    unmute: 'Bunyikan',
    explainer: 'Orang yang diblokir tidak bisa melihat atau menghubungimu di mana pun. Orang yang dibisukan cuma hilang dari feed-mu.',
  },
  contacts: {
    title: 'Kontak tepercaya',
    intro: 'Saat kamu menekan SOS, mereka dapat SMS berisi link live yang menunjukkan stasiun terakhir yang kamu lewati. Mereka tidak pernah melihat chat-mu.',
    remove: 'Hapus {name}',
    name: 'Nama',
    namePlaceholder: 'Kakak',
    mobile: 'Nomor HP',
    add: 'Tambah kontak',
    failed: 'Gagal',
  },
  privacy: {
    title: 'Privasi & datamu',
    allowed: 'Yang kamu izinkan',
    corePause: 'Mematikan ini akan menjeda akunmu.',
  },
  consent: {
    core_service: {
      title: 'Jalankan akunku',
      body: 'Simpan nomor HP-ku (terenkripsi), persona anonimku, dan postinganku agar aplikasi berfungsi. Wajib.',
    },
    travel_matching: {
      title: 'Pencocokan Travel Buddy',
      body: 'Pakai stasiun dan jam perjalananku untuk menyarankan orang di rute yang sama. Hanya bagian perjalanan yang sama yang ditampilkan.',
    },
    ai_processing: {
      title: 'Cek keamanan AI',
      body: 'Izinkan AI mengecek ulang pesanku untuk nomor HP atau akun medsos tersembunyi sebelum dikirim. AI tidak pernah bertindak sendiri.',
    },
    analytics: {
      title: 'Statistik penggunaan anonim',
      body: 'Hitung fitur mana yang dipakai, tanpa terhubung ke aku atau personaku.',
    },
    marketing_push: {
      title: 'Kabar produk',
      body: 'Sesekali notifikasi tentang fitur baru dan acara di kotaku.',
    },
  },
  profile: {
    avatar: 'Ganti avatar',
    avatarSaved: 'Avatar diperbarui',
    title: 'Edit profil',
    saved: 'Profil tersimpan',
    becomeTitle: 'Jadi {alias}?',
    becomeBody: 'Kamu bisa ganti alias sekali setiap 30 hari.',
    change: 'Ganti',
    aliasFailed: 'Gagal mengganti alias',
    newAlias: 'Ganti username',
    aliasChanged: 'Username diganti',
    aliasAvailable: 'Alias baru tersedia {date}',
    bio: 'Bio',
    hereFor: 'Di sini untuk',
    stations: 'Stasiun (privat)',
    nearHome: 'Dekat rumah',
    nearWork: 'Dekat kantor / kampus',
    notSet: 'Belum diatur',
    vibes: 'Vibe ({count})',
  },
  reveal: {
    title: 'Kartu reveal',
    saved: 'Tersimpan secara privat',
    intro:
      'Ini tetap terenkripsi dan tersembunyi. Di chat empat mata, kamu dan lawan bicaramu bisa memilih untuk membuka satu langkah demi satu: nama depan, lalu foto, lalu kontak. Tidak ada yang ditampilkan sampai kalian berdua setuju, dan kamu bisa mundur kapan saja.',
    firstName: 'Nama depan',
    firstNamePlaceholder: 'Cukup nama depanmu',
    photo: 'Foto',
    changePhoto: 'Ganti foto',
    addPhoto: 'Tambah foto',
    contact: 'Kontak',
    contactPlaceholder: 'Nomor HP atau akun medsos',
  },
};

export default settings;
