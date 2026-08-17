import {
  BadRequestException,
  Body,
  Controller,
  Delete,
  Get,
  Param,
  Patch,
  Post,
  Put,
  Query,
  UploadedFiles,
  UseGuards,
  UseInterceptors,
} from '@nestjs/common';
import { FileFieldsInterceptor } from '@nestjs/platform-express';
import { diskStorage } from 'multer';
import { mkdirSync } from 'fs';
import { extname, join } from 'path';
import {
  ApiBearerAuth,
  ApiBody,
  ApiConsumes,
  ApiOperation,
  ApiQuery,
  ApiTags,
} from '@nestjs/swagger';
import { Category } from '@prisma/client';
import { ProductsService } from './products.service';
import { CreateProductDTO } from './dtos/create-product.dto';
import { UpdateProductDTO } from './dtos/update-product.dto';
import { JwtAdminGuard } from '../jwt/jwt-admin.guard';

mkdirSync(join(process.cwd(), 'uploads'), { recursive: true });
mkdirSync(join(process.cwd(), 'files'), { recursive: true });

const storage = diskStorage({
  destination: (_req, file, cb) => {
    cb(null, file.fieldname === 'file' ? './files' : './uploads');
  },
  filename: (_, file, cb) => {
    const unique = `${Date.now()}-${Math.round(Math.random() * 1e9)}`;
    cb(null, `${unique}${extname(file.originalname)}`);
  },
});

const ALLOWED_COVER_EXTENSIONS = ['.jpg', '.jpeg', '.png'];
const ALLOWED_FILE_EXTENSIONS = ['.pdf', '.zip'];

const fileFilter = (
  _req: any,
  file: Express.Multer.File,
  cb: (error: Error | null, acceptFile: boolean) => void,
) => {
  const extension = extname(file.originalname).toLowerCase();
  const allowed =
    file.fieldname === 'cover'
      ? ALLOWED_COVER_EXTENSIONS
      : ALLOWED_FILE_EXTENSIONS;

  if (allowed.includes(extension)) {
    return cb(null, true);
  }

  cb(
    new BadRequestException(
      `Tipo de arquivo não permitido para "${file.fieldname}": ${file.originalname}`,
    ),
    false,
  );
};

@ApiTags('Products')
@Controller('products')
export class ProductsController {
  constructor(private readonly productsService: ProductsService) {}

  @Get()
  @ApiOperation({ summary: 'List all products' })
  @ApiQuery({ name: 'category', enum: Category, required: false })
  @ApiQuery({ name: 'isActive', type: Boolean, required: false })
  findAll(
    @Query('category') category?: Category,
    @Query('isActive') isActive?: string,
  ) {
    const parsedIsActive =
      isActive !== undefined ? isActive === 'true' : undefined;
    return this.productsService.findAll(category, parsedIsActive);
  }

  @Get(':id')
  @ApiOperation({ summary: 'Get product by id' })
  findOne(@Param('id') id: string) {
    return this.productsService.findOne(id);
  }

  @Post()
  @ApiOperation({ summary: 'Create a new product' })
  @ApiBearerAuth()
  @UseGuards(JwtAdminGuard)
  @ApiConsumes('multipart/form-data')
  @ApiBody({
    schema: {
      type: 'object',
      properties: {
        title: { type: 'string' },
        description: { type: 'string' },
        price: { type: 'number' },
        isActive: { type: 'boolean' },
        category: { type: 'string', enum: Object.values(Category) },
        cover: { type: 'string', format: 'binary' },
        file: { type: 'string', format: 'binary' },
      },
    },
  })
  @UseInterceptors(
    FileFieldsInterceptor(
      [
        { name: 'cover', maxCount: 1 },
        { name: 'file', maxCount: 1 },
      ],
      { storage, fileFilter },
    ),
  )
  create(
    @Body() dto: CreateProductDTO,
    @UploadedFiles()
    files: { cover?: Express.Multer.File[]; file?: Express.Multer.File[] },
  ) {
    const coverUrl = files.cover?.[0]?.filename ?? '';
    const fileUrl = files.file?.[0]?.filename ?? '';
    return this.productsService.create(dto, coverUrl, fileUrl);
  }

  @Put(':id')
  @ApiOperation({ summary: 'Update a product' })
  @ApiBearerAuth()
  @UseGuards(JwtAdminGuard)
  @ApiConsumes('multipart/form-data')
  @ApiBody({
    schema: {
      type: 'object',
      properties: {
        title: { type: 'string' },
        description: { type: 'string' },
        price: { type: 'number' },
        isActive: { type: 'boolean' },
        category: { type: 'string', enum: Object.values(Category) },
        cover: { type: 'string', format: 'binary' },
        file: { type: 'string', format: 'binary' },
      },
    },
  })
  @UseInterceptors(
    FileFieldsInterceptor(
      [
        { name: 'cover', maxCount: 1 },
        { name: 'file', maxCount: 1 },
      ],
      { storage, fileFilter },
    ),
  )
  update(
    @Param('id') id: string,
    @Body() dto: UpdateProductDTO,
    @UploadedFiles()
    files: { cover?: Express.Multer.File[]; file?: Express.Multer.File[] },
  ) {
    const coverUrl = files.cover?.[0]?.filename;
    const fileUrl = files.file?.[0]?.filename;
    return this.productsService.update(id, dto, coverUrl, fileUrl);
  }

  @Patch(':id/toggle')
  @ApiOperation({ summary: 'Toggle product isActive status' })
  @ApiBearerAuth()
  @UseGuards(JwtAdminGuard)
  toggle(@Param('id') id: string) {
    return this.productsService.toggle(id);
  }

  @Delete(':id')
  @ApiOperation({ summary: 'Delete a product' })
  @ApiBearerAuth()
  @UseGuards(JwtAdminGuard)
  remove(@Param('id') id: string) {
    return this.productsService.remove(id);
  }
}
